package notify

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/hilman2/ELNSSM/internal/config"
	"github.com/hilman2/ELNSSM/internal/model"
)

type mockNotifier struct {
	name      string
	events    []model.Event
	sendError error
}

func (m *mockNotifier) Name() string { return m.name }
func (m *mockNotifier) Send(event model.Event) error {
	m.events = append(m.events, event)
	return m.sendError
}

func TestDispatcher_NoNotifiers(t *testing.T) {
	cfg := config.DefaultConfig()
	d := NewDispatcher(cfg)
	// Should not panic
	d.Dispatch(model.Event{Type: model.EventServiceCrashed, ServiceID: "test"})
}

func TestDispatcher_SendsToAll(t *testing.T) {
	cfg := config.DefaultConfig()
	d := NewDispatcher(cfg)

	n1 := &mockNotifier{name: "n1"}
	n2 := &mockNotifier{name: "n2"}
	d.notifiers = []Notifier{n1, n2}

	event := model.Event{Type: model.EventServiceCrashed, ServiceID: "test", Timestamp: time.Now()}
	d.Dispatch(event)

	if len(n1.events) != 1 {
		t.Errorf("n1 received %d events, want 1", len(n1.events))
	}
	if len(n2.events) != 1 {
		t.Errorf("n2 received %d events, want 1", len(n2.events))
	}
}

func TestDispatcher_Cooldown(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.NotifyCooldown = "1h" // 1 hour cooldown
	d := NewDispatcher(cfg)

	n := &mockNotifier{name: "test"}
	d.notifiers = []Notifier{n}

	event := model.Event{Type: model.EventServiceCrashed, ServiceID: "test", Timestamp: time.Now()}
	d.Dispatch(event)
	d.Dispatch(event) // Should be suppressed

	if len(n.events) != 1 {
		t.Errorf("received %d events, want 1 (second should be suppressed)", len(n.events))
	}
}

func TestDispatcher_CooldownByServiceAndType(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.NotifyCooldown = "1h"
	d := NewDispatcher(cfg)

	n := &mockNotifier{name: "test"}
	d.notifiers = []Notifier{n}

	d.Dispatch(model.Event{Type: model.EventServiceCrashed, ServiceID: "svc-a", Timestamp: time.Now()})
	d.Dispatch(model.Event{Type: model.EventServiceCrashed, ServiceID: "svc-b", Timestamp: time.Now()}) // different service
	d.Dispatch(model.Event{Type: model.EventServiceStarted, ServiceID: "svc-a", Timestamp: time.Now()}) // different type

	if len(n.events) != 3 {
		t.Errorf("received %d events, want 3", len(n.events))
	}
}

func TestDispatcher_NotifierError(t *testing.T) {
	cfg := config.DefaultConfig()
	d := NewDispatcher(cfg)

	n := &mockNotifier{name: "failing", sendError: http.ErrAbortHandler}
	d.notifiers = []Notifier{n}

	// Should not panic despite notifier error
	d.Dispatch(model.Event{Type: model.EventServiceCrashed, ServiceID: "test", Timestamp: time.Now()})
}

func TestWebhook_EventFilter(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	}))
	defer srv.Close()

	wh := NewWebhookNotifier(config.WebhookConfig{
		Name:    "test",
		Enabled: true,
		URL:     srv.URL,
		Events:  []string{"service.crashed"},
	})

	// This event should be skipped
	err := wh.Send(model.Event{Type: model.EventServiceStarted, ServiceID: "test"})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestWebhook_Send(t *testing.T) {
	var receivedBody string
	var receivedHeaders http.Header
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedHeaders = r.Header
		buf := make([]byte, 1024)
		n, _ := r.Body.Read(buf)
		receivedBody = string(buf[:n])
		w.WriteHeader(200)
	}))
	defer srv.Close()

	wh := NewWebhookNotifier(config.WebhookConfig{
		Name:    "test",
		Enabled: true,
		URL:     srv.URL,
		Headers: map[string]string{"X-Custom": "test-value"},
		Events:  []string{"service.crashed"},
	})

	err := wh.Send(model.Event{
		Type:      model.EventServiceCrashed,
		ServiceID: "my-app",
		Message:   "process crashed",
		Timestamp: time.Now(),
	})
	if err != nil {
		t.Fatalf("Send: %v", err)
	}

	if receivedHeaders.Get("X-Custom") != "test-value" {
		t.Errorf("X-Custom header = %q, want test-value", receivedHeaders.Get("X-Custom"))
	}
	if receivedBody == "" {
		t.Error("body should not be empty")
	}
}

func TestWebhook_ErrorStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
	}))
	defer srv.Close()

	wh := NewWebhookNotifier(config.WebhookConfig{
		Name:    "test",
		Enabled: true,
		URL:     srv.URL,
	})

	err := wh.Send(model.Event{Type: model.EventServiceCrashed, ServiceID: "test", Timestamp: time.Now()})
	if err == nil {
		t.Error("expected error for 500 response")
	}
}

func TestWebhook_BodyTemplate(t *testing.T) {
	var receivedBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		buf := make([]byte, 4096)
		n, _ := r.Body.Read(buf)
		receivedBody = string(buf[:n])
		w.WriteHeader(200)
	}))
	defer srv.Close()

	wh := NewWebhookNotifier(config.WebhookConfig{
		Name:         "test",
		Enabled:      true,
		URL:          srv.URL,
		BodyTemplate: `{"event":"{{.Event}}","service":"{{.ServiceName}}"}`,
		Events:       []string{"service.crashed"},
	})

	wh.Send(model.Event{
		Type:      model.EventServiceCrashed,
		ServiceID: "my-app",
		Timestamp: time.Now(),
	})

	expected := `{"event":"service.crashed","service":"my-app"}`
	if receivedBody != expected {
		t.Errorf("body = %q, want %q", receivedBody, expected)
	}
}
