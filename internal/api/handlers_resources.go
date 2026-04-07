package api

import (
	"log/slog"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
)

// Maximum concurrent resource stream WebSocket connections.
const maxResourceStreamConns = 20

// Active resource stream connection counter.
var activeResourceConns int64

// handleStreamResources streams host and per-service resource metrics via WebSocket.
func (s *Server) handleStreamResources(w http.ResponseWriter, r *http.Request) {
	// Enforce connection limit to prevent goroutine/resource exhaustion
	if atomic.LoadInt64(&activeResourceConns) >= maxResourceStreamConns {
		writeError(w, http.StatusServiceUnavailable, "CONN_LIMIT", "Too many resource stream connections")
		return
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	atomic.AddInt64(&activeResourceConns, 1)
	defer func() {
		conn.Close()
		atomic.AddInt64(&activeResourceConns, -1)
	}()

	// Limit read size — reader goroutine only detects disconnects, no real data expected
	conn.SetReadLimit(512)

	// Reader goroutine for disconnect detection
	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			if _, _, err := conn.NextReader(); err != nil {
				return
			}
		}
	}()

	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	// Send an initial snapshot immediately
	conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
	if err := s.sendResourceSnapshot(conn); err != nil {
		return
	}

	for {
		select {
		case <-done:
			return
		case <-ticker.C:
			conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := s.sendResourceSnapshot(conn); err != nil {
				slog.Debug("Resource stream write error", "error", err)
				return
			}
		}
	}
}

type resourceSnapshot struct {
	Host     hostMetrics      `json:"host"`
	Services []serviceMetrics `json:"services"`
}

type hostMetrics struct {
	CPUPercent    float64 `json:"cpu_percent"`
	MemoryTotal   int64   `json:"memory_total"`
	MemoryUsed    int64   `json:"memory_used"`
	MemoryPercent float64 `json:"memory_percent"`
}

type serviceMetrics struct {
	ID          string  `json:"id"`
	CPUPercent  float64 `json:"cpu_percent"`
	MemoryBytes int64   `json:"memory_bytes"`
	State       string  `json:"state"`
}

func (s *Server) sendResourceSnapshot(conn *websocket.Conn) error {
	snapshot := resourceSnapshot{}

	// Host metrics
	if s.hostMonitor != nil {
		h := s.hostMonitor.Latest()
		snapshot.Host = hostMetrics{
			CPUPercent:    h.CPUPercent,
			MemoryTotal:   h.MemoryTotal,
			MemoryUsed:    h.MemoryUsed,
			MemoryPercent: h.MemoryPercent,
		}
	}

	// Per-service metrics
	services := s.manager.List()
	for _, svc := range services {
		snapshot.Services = append(snapshot.Services, serviceMetrics{
			ID:          svc.ID,
			CPUPercent:  svc.CPUPercent,
			MemoryBytes: svc.MemoryBytes,
			State:       string(svc.State),
		})
	}

	return conn.WriteJSON(snapshot)
}
