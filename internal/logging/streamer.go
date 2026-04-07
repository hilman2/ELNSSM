package logging

import (
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
)

// Maximum concurrent WebSocket streaming connections (log + event streams).
const maxStreamConns = 50

// Active streaming connection counter.
var activeStreamConns int64

// LogMessage is a single log line sent to WebSocket clients.
type LogMessage struct {
	ServiceID string `json:"service_id"`
	Stream    string `json:"stream"`
	Line      string `json:"line"`
}

// Client represents a connected WebSocket log viewer.
type Client struct {
	conn      *websocket.Conn
	serviceID string
	stream    string // "stdout", "stderr", "combined"
	send      chan LogMessage
}

// Streamer is a WebSocket hub for live log streaming.
type Streamer struct {
	clients    map[*Client]bool
	register   chan *Client
	unregister chan *Client
	mu         sync.RWMutex
}

// NewStreamer creates a new log streamer hub.
func NewStreamer() *Streamer {
	s := &Streamer{
		clients:    make(map[*Client]bool),
		register:   make(chan *Client),
		unregister: make(chan *Client),
	}
	go s.run()
	return s
}

func (s *Streamer) run() {
	for {
		select {
		case client := <-s.register:
			s.mu.Lock()
			s.clients[client] = true
			s.mu.Unlock()

		case client := <-s.unregister:
			s.mu.Lock()
			if _, ok := s.clients[client]; ok {
				delete(s.clients, client)
				close(client.send)
			}
			s.mu.Unlock()
		}
	}
}

// AtConnLimit returns true if the maximum number of streaming connections is reached.
func (s *Streamer) AtConnLimit() bool {
	return atomic.LoadInt64(&activeStreamConns) >= maxStreamConns
}

// Register adds a WebSocket client for log streaming.
func (s *Streamer) Register(conn *websocket.Conn, serviceID, stream string) *Client {
	atomic.AddInt64(&activeStreamConns, 1)

	// Harden connection: limit reads (only used for disconnect detection)
	// and set write deadlines to prevent slowloris-style hangs.
	conn.SetReadLimit(512)

	client := &Client{
		conn:      conn,
		serviceID: serviceID,
		stream:    stream,
		send:      make(chan LogMessage, 256),
	}
	s.register <- client

	// Start writer goroutine
	go func() {
		defer func() {
			s.unregister <- client
			conn.Close()
			atomic.AddInt64(&activeStreamConns, -1)
		}()

		for msg := range client.send {
			conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := conn.WriteJSON(msg); err != nil {
				slog.Debug("WebSocket write error", "error", err)
				return
			}
		}
	}()

	// Start reader goroutine (just to detect disconnects)
	go func() {
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				s.unregister <- client
				break
			}
		}
	}()

	return client
}

// Broadcast sends a log line to all matching clients.
func (s *Streamer) Broadcast(serviceID, stream, line string) {
	msg := LogMessage{
		ServiceID: serviceID,
		Stream:    stream,
		Line:      line,
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	for client := range s.clients {
		if client.serviceID != serviceID {
			continue
		}
		if client.stream != "combined" && client.stream != stream {
			continue
		}

		select {
		case client.send <- msg:
		default:
			// Client too slow, skip
		}
	}
}
