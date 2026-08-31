// Package cluster implements the master/agent clustering layer for
// running ELNSSM across multiple nodes with a single management entry
// point. It handles node registration, heartbeating and request proxying.
package cluster

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/hilman2/ELNSSM/internal/buildinfo"
	"github.com/hilman2/ELNSSM/internal/config"
)

// Heartbeat is the payload a slave posts to the master's heartbeat endpoint.
//
// ListenPort carries the API port of the sending node. It has to be in the
// payload because the master cannot observe it: the heartbeat reaches it from
// an ephemeral source port. The master pairs this port with the peer IP it
// saw, and never takes the host from the payload or a header, which is what
// keeps the proxy target from being pointed at an unrelated machine.
type Heartbeat struct {
	Name       string `json:"name"`
	Version    string `json:"version"`
	ListenPort int    `json:"listen_port"`
}

// SlaveNode represents a connected slave ELNSSM instance.
type SlaveNode struct {
	Name string `json:"name"`
	// Address is "host:port" and is the target ProxyRequest dials. The host
	// half always comes from the TCP peer address of the heartbeat, never
	// from a header, so a caller cannot point it at a third party. It is
	// empty when the slave did not report a usable listen port; see
	// RegisterHeartbeat.
	Address  string    `json:"address"`
	Status   string    `json:"status"` // "connected", "disconnected", "error"
	LastSeen time.Time `json:"last_seen"`
	Version  string    `json:"version"`
}

// Manager handles cluster communication between Master and Slave nodes.
type Manager struct {
	cfg    *config.ClusterConfig
	client *http.Client

	// listenPort is this node's own API port, reported to the master in each
	// heartbeat. The master cannot derive it: the heartbeat arrives from an
	// ephemeral source port, not from the port the API listens on.
	listenPort int

	// Master: tracks connected slaves
	slaves map[string]*SlaveNode
	mu     sync.RWMutex

	// Slave: heartbeat management
	cancel context.CancelFunc
}

// New creates a new cluster manager. listenAddr is this node's own API listen
// address in "host:port" form, as configured in api.listen; a port that cannot
// be parsed from it leaves the heartbeat without one, and the master will then
// refuse to proxy to this node.
func New(cfg *config.ClusterConfig, listenAddr string) *Manager {
	port := 0
	if _, portStr, err := net.SplitHostPort(listenAddr); err == nil {
		if p, convErr := strconv.Atoi(portStr); convErr == nil {
			port = p
		}
	}
	if port == 0 {
		slog.Warn("Could not determine own API port, master will not be able to proxy to this node",
			"listen", listenAddr)
	}

	return &Manager{
		cfg: cfg,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
		listenPort: port,
		slaves:     make(map[string]*SlaveNode),
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

// RegisterHeartbeat processes a heartbeat from a slave. peerIP must be the IP
// of the TCP peer the heartbeat arrived from, and listenPort the port the slave
// reported in its payload; the two are joined into the proxy target address.
//
// Passing anything caller-controlled as peerIP reopens a server-side request
// forgery: the address ends up in the URL that ProxyRequest dials, and the
// cluster's slave token travels with it. A listenPort outside 1-65535 leaves
// the node registered but without an address, so it stays visible in the node
// list while ProxyRequest refuses it.
func (m *Manager) RegisterHeartbeat(name, peerIP string, listenPort int, version string) {
	address := ""
	if listenPort > 0 && listenPort <= 65535 {
		address = net.JoinHostPort(peerIP, strconv.Itoa(listenPort))
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// A name arriving from a new address is worth a line in the log: names
	// are chosen by the sender, so this is what taking over another node's
	// entry would look like.
	if prev, ok := m.slaves[name]; ok && prev.Address != address {
		slog.Warn("Slave address changed", "name", name, "old", prev.Address, "new", address)
	}

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

	// Refuse a node whose address never got a port, and re-check the shape of
	// the address before interpolating it. RegisterHeartbeat already builds it
	// from a peer IP and a bounded port, so this only holds if that invariant
	// is broken later; a bare "host:port" cannot smuggle a path or another
	// host into the URL below.
	if node.Address == "" {
		return nil, http.StatusServiceUnavailable,
			fmt.Errorf("slave %q reported no listen port, cannot proxy", nodeName)
	}
	if _, _, err := net.SplitHostPort(node.Address); err != nil {
		return nil, http.StatusServiceUnavailable,
			fmt.Errorf("slave %q has malformed address %q", nodeName, node.Address)
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
	heartbeat := Heartbeat{
		Name:       m.cfg.NodeName,
		Version:    buildinfo.Version,
		ListenPort: m.listenPort,
	}

	data, err := json.Marshal(heartbeat)
	if err != nil {
		slog.Warn("Failed to encode heartbeat", "error", err)
		return
	}
	url := fmt.Sprintf("http://%s/api/v1/cluster/heartbeat", m.cfg.MasterAddr)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(data))
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
