package api

import (
	"log/slog"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
)

func (s *Server) handleGetHealth(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if !validateServiceID(w, id) {
		return
	}

	ms, ok := s.manager.Get(id)
	if !ok {
		writeError(w, http.StatusNotFound, "SERVICE_NOT_FOUND", "Service not found")
		return
	}

	// Get latest health check results
	history, err := s.store.GetHealthHistory(r.Context(), id, 1)
	if err != nil {
		slog.Error("Failed to get health history", "service", id, "error", err)
		writeError(w, http.StatusInternalServerError, "STORE_ERROR", "Failed to retrieve health data")
		return
	}

	result := map[string]any{
		"service_id":    id,
		"state":         ms.Config.State,
		"health_checks": ms.Config.HealthChecks,
		"latest":        nil,
	}

	if len(history) > 0 {
		result["latest"] = history[len(history)-1]
	}

	writeJSON(w, http.StatusOK, result)
}

func (s *Server) handleGetHealthHistory(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if !validateServiceID(w, id) {
		return
	}

	limit := 50
	if l := r.URL.Query().Get("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil && n > 0 {
			limit = n
		}
	}

	history, err := s.store.GetHealthHistory(r.Context(), id, limit)
	if err != nil {
		slog.Error("Failed to get health history", "service", id, "error", err)
		writeError(w, http.StatusInternalServerError, "STORE_ERROR", "Failed to retrieve health data")
		return
	}

	writeJSONList(w, history, len(history))
}
