package api

import (
	"log/slog"
	"net/http"
	"os/exec"
	"runtime"
	"time"

	"github.com/hilman2/ELNSSM/internal/buildinfo"
	"github.com/hilman2/ELNSSM/internal/store"
)

var startTime = time.Now()

func (s *Server) handleSystemStatus(w http.ResponseWriter, r *http.Request) {
	services := s.manager.List()
	running := 0
	failed := 0
	for _, svc := range services {
		switch svc.State {
		case "running":
			running++
		case "failed":
			failed++
		}
	}

	resp := map[string]any{
		"version":          buildinfo.Version,
		"uptime":           time.Since(startTime).Truncate(time.Second).String(),
		"started_at":       startTime.Format(time.RFC3339),
		"services_total":   len(services),
		"services_running": running,
		"services_failed":  failed,
		"go_version":       runtime.Version(),
		"os":               runtime.GOOS,
		"arch":             runtime.GOARCH,
	}

	if s.hostMonitor != nil {
		h := s.hostMonitor.Latest()
		resp["host_cpu_percent"] = h.CPUPercent
		resp["host_memory_total"] = h.MemoryTotal
		resp["host_memory_used"] = h.MemoryUsed
		resp["host_memory_percent"] = h.MemoryPercent
	}

	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleVersion(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{
		"version":    buildinfo.Version,
		"commit":     buildinfo.Commit,
		"build_date": buildinfo.BuildDate,
	})
}

func (s *Server) handleRestart(w http.ResponseWriter, r *http.Request) {
	if s.restarter == nil {
		writeError(w, http.StatusServiceUnavailable, "NOT_AVAILABLE", "Restart not available in this mode")
		return
	}

	slog.Info("Guardian restart requested via API")

	// Set restart mode so children are detached instead of killed
	s.restarter.RequestRestart()

	// Tell the SCM to restart the ELNSSM service.
	// sc.exe stop triggers the SCM stop -> Guardian detaches children -> SCM auto-restarts.
	// The service recovery options or a simple start-after-stop handles the restart.
	go func() {
		// Small delay to let the HTTP response be sent
		time.Sleep(500 * time.Millisecond)

		// Use net stop + net start via a helper cmd to restart the Windows service
		cmd := exec.Command("cmd", "/C", "net stop ELNSSM && net start ELNSSM")
		if err := cmd.Start(); err != nil {
			slog.Error("Failed to trigger service restart via SCM", "error", err)
		}
	}()

	writeJSON(w, http.StatusOK, map[string]string{
		"message": "Guardian restart initiated. Children will continue running and be re-adopted.",
	})
}

func (s *Server) handleListEvents(w http.ResponseWriter, r *http.Request) {
	filter := store.EventFilter{
		Limit: 100,
	}

	if sid := r.URL.Query().Get("service_id"); sid != "" {
		filter.ServiceID = &sid
	}

	events, err := s.store.ListEvents(r.Context(), filter)
	if err != nil {
		slog.Error("Failed to list events", "error", err)
		writeError(w, http.StatusInternalServerError, "STORE_ERROR", "Failed to retrieve events")
		return
	}

	writeJSONList(w, events, len(events))
}
