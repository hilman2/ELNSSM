package cluster

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/hilman2/ELNSSM/internal/buildinfo"
	"github.com/hilman2/ELNSSM/internal/config"
)

// SlaveNode represents a connected slave ELNSSM instance.
type SlaveNode struct {
	Name     string    `json:"name"`
	Address  string    `json:"address"`
	Status   string    `json:"status"` // "connected", "disconnected", "error"
	LastSeen time.Time `json:"last_seen"`
	Version  string    `json:"version"`
}

// Manager handles cluster communication between Master and Slave nodes.
type Manager struct {
	cfg    *config.ClusterConfig
	client *http.Client

	// Master: tracks connected slaves
	slaves map[string]*SlaveNode
	mu     sync.RWMutex

	// Slave: heartbeat management
	cancel context.CancelFunc
}

// New creates a new cluster manager.
func New(cfg *config.ClusterConfig) *Manager {
	return &Manager{
		cfg: cfg,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
		slaves: make(map[string]*SlaveNode),
	}
}

// Role returns the current cluster role.
func (m *Manager) Role() string {
	if m.cfg.Role == "" {
		return "standalone"
	}
	return m.cfg.Role
}

// IsMaster returns true if this node is a master.
func (m *Manager) IsMaster() bool {
	return m.cfg.Role == "master"
}

// IsSlave returns true if this node is a slave.
func (m *Manager) IsSlave() bool {
	return m.cfg.Role == "slave"
}

// Start begins cluster operations (heartbeat for slaves).
func (m *Manager) Start(ctx context.Context) {
	if m.IsSlave() && m.cfg.MasterAddr != "" {
		ctx, m.cancel = context.WithCancel(ctx)
		go m.heartbeatLoop(ctx)
	}
}

// Stop stops cluster operations.
func (m *Manager) Stop() {
	if m.cancel != nil {
		m.cancel()
	}
}

// --- Master operations ---

// RegisterHeartbeat processes a heartbeat from a slave.
func (m *Manager) RegisterHeartbeat(name, address, version string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.slaves[name] = &SlaveNode{
		Name:     name,
		Address:  address,
		Status:   "connected",
		LastSeen: time.Now(),
		Version:  version,
	}
	slog.Info("Slave heartbeat received", "name", name, "address", address)
}

// ListSlaves returns all known slave nodes.
func (m *Manager) ListSlaves() []SlaveNode {
	m.mu.RLock()
	defer m.mu.RUnlock()

	now := time.Now()
	result := make([]SlaveNode, 0, len(m.slaves))
	for _, node := range m.slaves {
		n := *node
		// Mark as disconnected if no heartbeat for 2 minutes
		if now.Sub(n.LastSeen) > 2*time.Minute {
			n.Status = "disconnected"
		}
		result = append(result, n)
	}
	return result
}

// GetSlave returns info about a specific slave.
func (m *Manager) GetSlave(name string) (*SlaveNode, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	node, ok := m.slaves[name]
	if !ok {
		return nil, false
	}
	n := *node
	return &n, true
}

// ProxyRequest forwards an API request to a slave node.
func (m *Manager) ProxyRequest(ctx context.Context, nodeName, method, path string, body io.Reader) ([]byte, int, error) {
	m.mu.RLock()
	node, ok := m.slaves[nodeName]
	m.mu.RUnlock()

	if !ok {
		return nil, http.StatusNotFound, fmt.Errorf("slave %q not found", nodeName)
	}

	url := fmt.Sprintf("http://%s/api/v1%s", node.Address, path)
	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return nil, 0, fmt.Errorf("creating proxy request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	if m.cfg.SlaveToken != "" {
		req.Header.Set("Authorization", "Bearer "+m.cfg.SlaveToken)
	}

	resp, err := m.client.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("proxy request to %s: %w", node.Address, err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, fmt.Errorf("reading proxy response: %w", err)
	}

	return data, resp.StatusCode, nil
}

// --- Slave operations ---

// heartbeatLoop sends periodic heartbeats to the master.
func (m *Manager) heartbeatLoop(ctx context.Context) {
	interval := 30 * time.Second
	if m.cfg.HeartbeatInterval != "" {
		if d, err := time.ParseDuration(m.cfg.HeartbeatInterval); err == nil {
			interval = d
		}
	}

	slog.Info("Starting slave heartbeat", "master", m.cfg.MasterAddr, "interval", interval)

	// Send initial heartbeat immediately
	m.sendHeartbeat(ctx)

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			m.sendHeartbeat(ctx)
		case <-ctx.Done():
			return
		}
	}
}

func (m *Manager) sendHeartbeat(ctx context.Context) {
	heartbeat := map[string]string{
		"name":    m.cfg.NodeName,
		"version": buildinfo.Version,
	}

	data, _ := json.Marshal(heartbeat)
	url := fmt.Sprintf("http://%s/api/v1/cluster/heartbeat", m.cfg.MasterAddr)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, io.NopCloser(
		io.NewSectionReader(readerAt(data), 0, int64(len(data))),
	))
	if err != nil {
		slog.Warn("Failed to create heartbeat request", "error", err)
		return
	}

	req.Header.Set("Content-Type", "application/json")
	if m.cfg.MasterToken != "" {
		req.Header.Set("Authorization", "Bearer "+m.cfg.MasterToken)
	}

	resp, err := m.client.Do(req)
	if err != nil {
		slog.Warn("Failed to send heartbeat to master", "master", m.cfg.MasterAddr, "error", err)
		return
	}
	resp.Body.Close()

	if resp.StatusCode >= 400 {
		slog.Warn("Master rejected heartbeat", "status", resp.StatusCode)
	}
}

// readerAt wraps a byte slice to implement io.ReaderAt.
type readerAt []byte

func (r readerAt) ReadAt(p []byte, off int64) (n int, err error) {
	if off >= int64(len(r)) {
		return 0, io.EOF
	}
	n = copy(p, r[off:])
	if n < len(p) {
		err = io.EOF
	}
	return
}
