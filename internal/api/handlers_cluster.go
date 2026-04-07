package api

import (
	"encoding/json"
	"io"
	"net/http"

	"github.com/go-chi/chi/v5"
)

// handleClusterNodes lists all connected slave nodes (Master only).
func (s *Server) handleClusterNodes(w http.ResponseWriter, r *http.Request) {
	if s.cluster == nil || !s.cluster.IsMaster() {
		writeError(w, http.StatusBadRequest, "NOT_MASTER", "This node is not a cluster master")
		return
	}

	writeJSON(w, http.StatusOK, s.cluster.ListSlaves())
}

// handleClusterHeartbeat receives heartbeats from slave nodes (Master only).
func (s *Server) handleClusterHeartbeat(w http.ResponseWriter, r *http.Request) {
	if s.cluster == nil || !s.cluster.IsMaster() {
		writeError(w, http.StatusBadRequest, "NOT_MASTER", "This node is not a cluster master")
		return
	}

	var payload struct {
		Name    string `json:"name"`
		Version string `json:"version"`
	}

	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_BODY", "Invalid heartbeat payload")
		return
	}

	// Use the remote address as the slave's address
	remoteAddr := r.RemoteAddr
	if forwarded := r.Header.Get("X-Forwarded-For"); forwarded != "" {
		remoteAddr = forwarded
	}

	// Use the slave's listen address from config if sent, otherwise use remote addr
	slaveAddr := remoteAddr
	if addr := r.Header.Get("X-ELNSSM-Listen"); addr != "" {
		slaveAddr = addr
	}

	s.cluster.RegisterHeartbeat(payload.Name, slaveAddr, payload.Version)

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// handleClusterStatus returns the cluster status for this node.
func (s *Server) handleClusterStatus(w http.ResponseWriter, r *http.Request) {
	role := "standalone"
	if s.cluster != nil {
		role = s.cluster.Role()
	}

	result := map[string]any{
		"role":      role,
		"node_name": s.cfg.Cluster.NodeName,
	}

	if s.cluster != nil && s.cluster.IsMaster() {
		result["slaves"] = s.cluster.ListSlaves()
	}

	writeJSON(w, http.StatusOK, result)
}

// handleClusterProxy proxies API requests to slave nodes (Master only).
func (s *Server) handleClusterProxy(w http.ResponseWriter, r *http.Request) {
	if s.cluster == nil || !s.cluster.IsMaster() {
		writeError(w, http.StatusBadRequest, "NOT_MASTER", "This node is not a cluster master")
		return
	}

	nodeID := chi.URLParam(r, "nodeId")

	if _, ok := s.cluster.GetSlave(nodeID); !ok {
		writeError(w, http.StatusNotFound, "NODE_NOT_FOUND", "Slave node not found")
		return
	}

	// Extract the remaining path after /cluster/nodes/{nodeId}/proxy/
	proxyPath := chi.URLParam(r, "*")
	if proxyPath == "" {
		proxyPath = "/"
	}
	proxyPath = "/" + proxyPath

	var body io.Reader
	if r.Body != nil {
		body = r.Body
		defer r.Body.Close()
	}

	data, statusCode, err := s.cluster.ProxyRequest(r.Context(), nodeID, r.Method, proxyPath, body)
	if err != nil {
		writeError(w, http.StatusBadGateway, "PROXY_ERROR", "Failed to proxy request: "+err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	w.Write(data)
}
