package api

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/hilman2/ELNSSM/internal/model"
	"github.com/hilman2/ELNSSM/internal/process"
)

func (s *Server) handleListServices(w http.ResponseWriter, r *http.Request) {
	services := s.manager.List()
	// Compute uptime for running services
	for _, svc := range services {
		if svc.State == model.ServiceStateRunning && svc.StartedAt != nil {
			svc.Uptime = time.Since(*svc.StartedAt)
		}
	}
	writeJSONList(w, services, len(services))
}

func (s *Server) handleGetService(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if !validateServiceID(w, id) {
		return
	}

	ms, ok := s.manager.Get(id)
	if !ok {
		writeError(w, http.StatusNotFound, "SERVICE_NOT_FOUND", "Service not found")
		return
	}

	svc := *ms.Config
	if svc.State == model.ServiceStateRunning && svc.StartedAt != nil {
		svc.Uptime = time.Since(*svc.StartedAt)
	}
	writeJSON(w, http.StatusOK, svc)
}

// serviceRequest extends model.Service with the password field for API requests.
type serviceRequest struct {
	model.Service
	ServiceAccountPassword string `json:"service_account_password,omitempty"`
}

func (s *Server) handleAddService(w http.ResponseWriter, r *http.Request) {
	var req serviceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_BODY", "Invalid request body")
		return
	}

	svc := &req.Service
	if svc.ID == "" || svc.Executable == "" {
		writeError(w, http.StatusBadRequest, "MISSING_FIELDS", "Fields 'id' and 'executable' are required")
		return
	}

	// Validate service ID from body
	if !validateServiceID(w, svc.ID) {
		return
	}

	// Validate health check script targets
	for _, hc := range svc.HealthChecks {
		if hc.Type == model.HealthCheckScript && hc.Target != "" && containsShellMeta(hc.Target) {
			writeError(w, http.StatusBadRequest, "INVALID_HEALTH_CHECK", "Health check script target contains shell metacharacters; use script_body instead")
			return
		}
	}

	// Encrypt service account password if provided
	if req.ServiceAccountPassword != "" {
		encrypted, err := process.EncryptPassword(req.ServiceAccountPassword)
		if err != nil {
			slog.Error("Failed to encrypt service account password", "error", err)
			writeError(w, http.StatusInternalServerError, "ENCRYPT_FAILED", "Failed to encrypt password")
			return
		}
		svc.ServiceAccountPassword = encrypted
	}

	// Apply defaults
	if svc.Name == "" {
		svc.Name = svc.ID
	}
	if svc.StopTimeout == 0 {
		svc.StopTimeout = 30 * time.Second
	}
	if svc.StopSignal == "" {
		svc.StopSignal = model.StopSignalCtrlC
	}
	if svc.Priority == "" {
		svc.Priority = model.PriorityNormal
	}
	if svc.StartupType == "" {
		svc.StartupType = model.StartupManual
	}
	if svc.RestartPolicy.Mode == "" {
		svc.RestartPolicy.Mode = model.RestartOnFailure
	}
	if svc.RestartPolicy.MaxRetries == 0 {
		svc.RestartPolicy.MaxRetries = 10
	}
	if svc.RestartPolicy.Delay == 0 {
		svc.RestartPolicy.Delay = 5 * time.Second
	}
	if svc.RestartPolicy.BackoffMultiplier == 0 {
		svc.RestartPolicy.BackoffMultiplier = 2.0
	}
	if svc.RestartPolicy.MaxBackoff == 0 {
		svc.RestartPolicy.MaxBackoff = 5 * time.Minute
	}

	if err := s.manager.Add(r.Context(), svc); err != nil {
		writeError(w, http.StatusConflict, "SERVICE_EXISTS", err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, svc)
}

func (s *Server) handleUpdateService(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if !validateServiceID(w, id) {
		return
	}

	var req serviceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_BODY", "Invalid request body")
		return
	}

	svc := &req.Service
	svc.ID = id

	// Validate health check script targets
	for _, hc := range svc.HealthChecks {
		if hc.Type == model.HealthCheckScript && hc.Target != "" && containsShellMeta(hc.Target) {
			writeError(w, http.StatusBadRequest, "INVALID_HEALTH_CHECK", "Health check script target contains shell metacharacters; use script_body instead")
			return
		}
	}

	// Encrypt service account password if provided
	if req.ServiceAccountPassword != "" {
		encrypted, err := process.EncryptPassword(req.ServiceAccountPassword)
		if err != nil {
			slog.Error("Failed to encrypt service account password", "error", err)
			writeError(w, http.StatusInternalServerError, "ENCRYPT_FAILED", "Failed to encrypt password")
			return
		}
		svc.ServiceAccountPassword = encrypted
	}

	if err := s.manager.Update(r.Context(), id, svc); err != nil {
		writeError(w, http.StatusNotFound, "SERVICE_NOT_FOUND", err.Error())
		return
	}

	writeJSON(w, http.StatusOK, svc)
}

func (s *Server) handleGetResources(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if !validateServiceID(w, id) {
		return
	}

	ms, ok := s.manager.Get(id)
	if !ok {
		writeError(w, http.StatusNotFound, "SERVICE_NOT_FOUND", "Service not found")
		return
	}

	response := map[string]any{
		"cpu_percent":     0.0,
		"memory_bytes":    int64(0),
		"resource_limits": ms.Config.ResourceLimits,
	}

	if ms.ResourceMonitor != nil {
		sample := ms.ResourceMonitor.Latest()
		response["cpu_percent"] = sample.CPUPercent
		response["memory_bytes"] = sample.MemoryBytes
	}

	writeJSON(w, http.StatusOK, response)
}

func (s *Server) handleDeleteService(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if !validateServiceID(w, id) {
		return
	}

	if err := s.manager.Remove(r.Context(), id); err != nil {
		writeError(w, http.StatusConflict, "DELETE_FAILED", err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"message": "Service removed"})
}

func (s *Server) handleStartService(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if !validateServiceID(w, id) {
		return
	}

	if err := s.manager.Start(r.Context(), id); err != nil {
		writeError(w, http.StatusConflict, "START_FAILED", err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"message": "Service started"})
}

func (s *Server) handleStopService(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if !validateServiceID(w, id) {
		return
	}

	if err := s.manager.Stop(r.Context(), id); err != nil {
		writeError(w, http.StatusConflict, "STOP_FAILED", err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"message": "Service stopped"})
}

func (s *Server) handleRestartService(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if !validateServiceID(w, id) {
		return
	}

	if err := s.manager.Restart(r.Context(), id); err != nil {
		writeError(w, http.StatusConflict, "RESTART_FAILED", err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"message": "Service restarted"})
}
