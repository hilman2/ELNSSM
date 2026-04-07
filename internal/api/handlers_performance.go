package api

import (
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
)

func (s *Server) handleGetPerformance(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if !validateServiceID(w, id) {
		return
	}

	if _, ok := s.manager.Get(id); !ok {
		writeError(w, http.StatusNotFound, "SERVICE_NOT_FOUND", "Service not found")
		return
	}

	// Parse query parameters
	now := time.Now()
	from := now.Add(-1 * time.Hour) // default: last hour
	to := now
	maxPoints := 200

	if v := r.URL.Query().Get("from"); v != "" {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			from = t
		}
	}
	if v := r.URL.Query().Get("to"); v != "" {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			to = t
		}
	}
	if v := r.URL.Query().Get("range"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			from = now.Add(-d)
			to = now
		}
	}
	if v := r.URL.Query().Get("points"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 1000 {
			maxPoints = n
		}
	}

	samples, err := s.store.GetResourceHistory(r.Context(), id, from, to, maxPoints)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "STORE_ERROR", "Failed to retrieve performance data")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"service_id": id,
		"from":       from,
		"to":         to,
		"points":     len(samples),
		"samples":    samples,
	})
}
