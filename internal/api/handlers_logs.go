package api

import (
	"bufio"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/gorilla/websocket"

	"github.com/hilman2/ELNSSM/internal/logging"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		origin := r.Header.Get("Origin")
		if origin == "" {
			return true // Same-origin or CLI client
		}
		return origin == "http://"+r.Host || origin == "https://"+r.Host
	},
}

func (s *Server) handleGetLogs(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if !validateServiceID(w, id) {
		return
	}

	stream := r.URL.Query().Get("stream")
	if stream == "" {
		stream = "stdout"
	}
	if !validateLogStream(w, stream) {
		return
	}

	linesStr := r.URL.Query().Get("lines")
	lines := 100
	if linesStr != "" {
		if n, err := strconv.Atoi(linesStr); err == nil && n > 0 {
			lines = n
		}
	}

	logFile := filepath.Join(s.cfg.ServiceLogDir(id), stream+".log")
	if _, err := os.Stat(logFile); os.IsNotExist(err) {
		writeError(w, http.StatusNotFound, "LOG_NOT_FOUND", "Log file not found")
		return
	}

	// Read last N lines
	content, err := tailFile(logFile, lines)
	if err != nil {
		slog.Error("Failed to read log file", "path", logFile, "error", err)
		writeError(w, http.StatusInternalServerError, "LOG_READ_ERROR", "Failed to read log file")
		return
	}

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(content))
}

func (s *Server) handleDownloadLog(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if !validateServiceID(w, id) {
		return
	}

	stream := r.URL.Query().Get("stream")
	if stream == "" {
		stream = "stdout"
	}
	if !validateLogStream(w, stream) {
		return
	}

	logFile := filepath.Join(s.cfg.ServiceLogDir(id), stream+".log")
	if _, err := os.Stat(logFile); os.IsNotExist(err) {
		writeError(w, http.StatusNotFound, "LOG_NOT_FOUND", "Log file not found")
		return
	}

	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s-%s.log", id, stream))
	http.ServeFile(w, r, logFile)
}

func (s *Server) handleStreamLogs(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if !validateServiceID(w, id) {
		return
	}

	stream := r.URL.Query().Get("stream")
	if stream == "" {
		stream = "combined"
	}
	if !validateLogStream(w, stream) {
		return
	}

	if s.streamer.AtConnLimit() {
		writeError(w, http.StatusServiceUnavailable, "CONN_LIMIT", "Too many streaming connections")
		return
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}

	s.streamer.Register(conn, id, stream)
}

// handleStreamEvents streams system events via WebSocket.
func (s *Server) handleStreamEvents(w http.ResponseWriter, r *http.Request) {
	if s.streamer.AtConnLimit() {
		writeError(w, http.StatusServiceUnavailable, "CONN_LIMIT", "Too many streaming connections")
		return
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}

	// Register for all events
	s.streamer.Register(conn, "*", "events")
}

// tailFile reads the last N lines from a file.
func tailFile(path string, n int) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	var lines []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
		if len(lines) > n {
			lines = lines[1:]
		}
	}

	result := ""
	for _, line := range lines {
		result += line + "\n"
	}
	return result, scanner.Err()
}

// Ensure Streamer is accessible for WebSocket log streaming
var _ = (*logging.Streamer)(nil)
