package cluster

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"

	"github.com/hilman2/ELNSSM/internal/config"
)

func masterManager(t *testing.T) *Manager {
	t.Helper()
	return New(&config.ClusterConfig{Role: "master"}, "127.0.0.1:9100")
}

func TestNewParsesOwnListenPort(t *testing.T) {
	tests := []struct {
		name       string
		listenAddr string
		want       int
	}{
		{"loopback", "127.0.0.1:9100", 9100},
		{"wildcard", "0.0.0.0:8080", 8080},
		{"all interfaces shorthand", ":9100", 9100},
		{"ipv6", "[::1]:9100", 9100},
		{"no port", "127.0.0.1", 0},
		{"empty", "", 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := New(&config.ClusterConfig{Role: "slave"}, tt.listenAddr)
			if m.listenPort != tt.want {
				t.Errorf("listenPort = %d, want %d", m.listenPort, tt.want)
			}
		})
	}
}

// The proxy target must be built from the peer IP the master itself observed.
// Taking a host from anywhere the caller controls is what allowed a heartbeat
// to aim the proxy, and the cluster's slave token, at an unrelated machine.
func TestRegisterHeartbeatBuildsAddressFromPeerIP(t *testing.T) {
	m := masterManager(t)
	m.RegisterHeartbeat("node1", "192.168.1.5", 9100, "1.0.0")

	node, ok := m.GetSlave("node1")
	if !ok {
		t.Fatal("node1 was not registered")
	}
	if node.Address != "192.168.1.5:9100" {
		t.Errorf("Address = %q, want %q", node.Address, "192.168.1.5:9100")
	}
}

func TestRegisterHeartbeatRejectsUnusablePort(t *testing.T) {
	tests := []struct {
		name string
		port int
	}{
		{"missing", 0},
		{"negative", -1},
		{"above range", 65536},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := masterManager(t)
			m.RegisterHeartbeat("node1", "192.168.1.5", tt.port, "1.0.0")

			node, ok := m.GetSlave("node1")
			if !ok {
				t.Fatal("node should stay registered so it remains visible in the node list")
			}
			if node.Address != "" {
				t.Errorf("Address = %q, want empty so ProxyRequest refuses it", node.Address)
			}
		})
	}
}

func TestProxyRequestRefusesNodeWithoutAddress(t *testing.T) {
	m := masterManager(t)
	m.RegisterHeartbeat("node1", "192.168.1.5", 0, "1.0.0")

	_, status, err := m.ProxyRequest(context.Background(), "node1", http.MethodGet, "/services", nil)
	if err == nil {
		t.Fatal("expected an error for a node with no address")
	}
	if status != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want %d", status, http.StatusServiceUnavailable)
	}
}

func TestProxyRequestUnknownNode(t *testing.T) {
	m := masterManager(t)

	_, status, err := m.ProxyRequest(context.Background(), "nope", http.MethodGet, "/services", nil)
	if err == nil {
		t.Fatal("expected an error for an unknown node")
	}
	if status != http.StatusNotFound {
		t.Errorf("status = %d, want %d", status, http.StatusNotFound)
	}
}

// A registered address reaches the URL only as host:port, so a value carrying
// a path or a second host cannot redirect the request elsewhere.
func TestProxyRequestRejectsMalformedAddress(t *testing.T) {
	m := masterManager(t)
	m.mu.Lock()
	m.slaves["evil"] = &SlaveNode{Name: "evil", Address: "attacker.example/x?a="}
	m.mu.Unlock()

	_, status, err := m.ProxyRequest(context.Background(), "evil", http.MethodGet, "/services", nil)
	if err == nil {
		t.Fatal("expected an error for a malformed address")
	}
	if status != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want %d", status, http.StatusServiceUnavailable)
	}
}

func TestProxyRequestForwardsToSlave(t *testing.T) {
	var gotPath, gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotAuth = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()

	host, portStr := splitHostPort(t, srv.URL)
	port, err := strconv.Atoi(portStr)
	if err != nil {
		t.Fatalf("parsing test server port: %v", err)
	}

	m := New(&config.ClusterConfig{Role: "master", SlaveToken: "s3cret"}, "127.0.0.1:9100")
	m.RegisterHeartbeat("node1", host, port, "1.0.0")

	data, status, err := m.ProxyRequest(context.Background(), "node1", http.MethodGet, "/services", nil)
	if err != nil {
		t.Fatalf("ProxyRequest: %v", err)
	}
	if status != http.StatusCreated {
		t.Errorf("status = %d, want %d", status, http.StatusCreated)
	}
	if string(data) != `{"ok":true}` {
		t.Errorf("body = %q", data)
	}
	if gotPath != "/api/v1/services" {
		t.Errorf("slave saw path %q, want %q", gotPath, "/api/v1/services")
	}
	if gotAuth != "Bearer s3cret" {
		t.Errorf("slave saw auth %q, want %q", gotAuth, "Bearer s3cret")
	}
}

func splitHostPort(t *testing.T, rawURL string) (host, port string) {
	t.Helper()
	u, err := url.Parse(rawURL)
	if err != nil {
		t.Fatalf("parsing %q: %v", rawURL, err)
	}
	host, port, found := strings.Cut(u.Host, ":")
	if !found {
		t.Fatalf("test server URL %q has no port", rawURL)
	}
	return host, port
}
