package api

import (
	"net/http"

	"github.com/hilman2/ELNSSM/internal/winservice"
	"github.com/go-chi/chi/v5"
)

func (s *Server) handleListNativeServices(w http.ResponseWriter, r *http.Request) {
	services, err := winservice.ListNativeServices()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "SCM_ERROR", "Failed to list Windows services: "+err.Error())
		return
	}

	writeJSON(w, http.StatusOK, services)
}

func (s *Server) handleGetNativeService(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")

	svcInfo, err := winservice.GetNativeService(name)
	if err != nil {
		writeError(w, http.StatusNotFound, "SERVICE_NOT_FOUND", "Windows service not found: "+err.Error())
		return
	}

	writeJSON(w, http.StatusOK, svcInfo)
}

func (s *Server) handleStartNativeService(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")

	if err := winservice.StartNativeService(name); err != nil {
		writeError(w, http.StatusInternalServerError, "START_FAILED", "Failed to start service: "+err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "started", "service": name})
}

func (s *Server) handleStopNativeService(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")

	if err := winservice.StopNativeService(name); err != nil {
		writeError(w, http.StatusInternalServerError, "STOP_FAILED", "Failed to stop service: "+err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "stopped", "service": name})
}

func (s *Server) handleRestartNativeService(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")

	if err := winservice.RestartNativeService(name); err != nil {
		writeError(w, http.StatusInternalServerError, "RESTART_FAILED", "Failed to restart service: "+err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "restarted", "service": name})
}
