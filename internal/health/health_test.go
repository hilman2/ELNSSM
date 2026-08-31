package health

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/hilman2/ELNSSM/internal/model"
)

// --- HTTPChecker Tests ---

func TestHTTPChecker_Healthy(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		w.Write([]byte("OK"))
	}))
	defer srv.Close()

	checker := NewHTTPChecker(model.HealthCheckConfig{
		Type:    model.HealthCheckHTTP,
		Target:  srv.URL,
		Timeout: 5 * time.Second,
	})

	result := checker.Check(context.Background())
	if result.Status != model.HealthStatusHealthy {
		t.Errorf("Status = %s, want healthy (message: %s)", result.Status, result.Message)
	}
	if result.StatusCode != 200 {
		t.Errorf("StatusCode = %d, want 200", result.StatusCode)
	}
	// Not "> 0": the Windows system clock advances in steps of roughly 15ms,
	// and a request to a local test server finishes well inside one step, so
	// time.Since can legitimately return zero. Asserting a positive duration
	// makes this test fail at random on the Windows runner.
	if result.Duration < 0 {
		t.Errorf("Duration = %v, want a non-negative measurement", result.Duration)
	}
}

func TestHTTPChecker_UnhealthyStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
	}))
	defer srv.Close()

	checker := NewHTTPChecker(model.HealthCheckConfig{
		Type:    model.HealthCheckHTTP,
		Target:  srv.URL,
		Timeout: 5 * time.Second,
	})

	result := checker.Check(context.Background())
	if result.Status != model.HealthStatusUnhealthy {
		t.Errorf("Status = %s, want unhealthy", result.Status)
	}
	if result.StatusCode != 500 {
		t.Errorf("StatusCode = %d, want 500", result.StatusCode)
	}
}

func TestHTTPChecker_CustomExpectStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(204)
	}))
	defer srv.Close()

	checker := NewHTTPChecker(model.HealthCheckConfig{
		Type:         model.HealthCheckHTTP,
		Target:       srv.URL,
		ExpectStatus: 204,
		Timeout:      5 * time.Second,
	})

	result := checker.Check(context.Background())
	if result.Status != model.HealthStatusHealthy {
		t.Errorf("Status = %s, want healthy (message: %s)", result.Status, result.Message)
	}
}

func TestHTTPChecker_BodyMatch(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"status":"ok","version":"1.2.3"}`))
	}))
	defer srv.Close()

	checker := NewHTTPChecker(model.HealthCheckConfig{
		Type:       model.HealthCheckHTTP,
		Target:     srv.URL,
		ExpectBody: `"status":"ok"`,
		Timeout:    5 * time.Second,
	})

	result := checker.Check(context.Background())
	if result.Status != model.HealthStatusHealthy {
		t.Errorf("Status = %s, want healthy (message: %s)", result.Status, result.Message)
	}
}

func TestHTTPChecker_BodyNoMatch(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"status":"error"}`))
	}))
	defer srv.Close()

	checker := NewHTTPChecker(model.HealthCheckConfig{
		Type:       model.HealthCheckHTTP,
		Target:     srv.URL,
		ExpectBody: `"status":"ok"`,
		Timeout:    5 * time.Second,
	})

	result := checker.Check(context.Background())
	if result.Status != model.HealthStatusUnhealthy {
		t.Errorf("Status = %s, want unhealthy", result.Status)
	}
}

func TestHTTPChecker_ConnectionRefused(t *testing.T) {
	checker := NewHTTPChecker(model.HealthCheckConfig{
		Type:    model.HealthCheckHTTP,
		Target:  "http://127.0.0.1:1", // unlikely to be listening
		Timeout: 1 * time.Second,
	})

	result := checker.Check(context.Background())
	if result.Status != model.HealthStatusUnhealthy {
		t.Errorf("Status = %s, want unhealthy", result.Status)
	}
}

func TestHTTPChecker_DefaultMethod(t *testing.T) {
	var receivedMethod string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedMethod = r.Method
		w.WriteHeader(200)
	}))
	defer srv.Close()

	checker := NewHTTPChecker(model.HealthCheckConfig{
		Type:    model.HealthCheckHTTP,
		Target:  srv.URL,
		Timeout: 5 * time.Second,
	})

	checker.Check(context.Background())
	if receivedMethod != "GET" {
		t.Errorf("Method = %q, want GET", receivedMethod)
	}
}

// --- TCPChecker Tests ---

func TestTCPChecker_Healthy(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Listen: %v", err)
	}
	defer ln.Close()

	checker := NewTCPChecker(model.HealthCheckConfig{
		Type:    model.HealthCheckTCP,
		Target:  ln.Addr().String(),
		Timeout: 5 * time.Second,
	})

	result := checker.Check(context.Background())
	if result.Status != model.HealthStatusHealthy {
		t.Errorf("Status = %s, want healthy (message: %s)", result.Status, result.Message)
	}
}

func TestTCPChecker_Unhealthy(t *testing.T) {
	checker := NewTCPChecker(model.HealthCheckConfig{
		Type:    model.HealthCheckTCP,
		Target:  "127.0.0.1:1", // unlikely to be listening
		Timeout: 1 * time.Second,
	})

	result := checker.Check(context.Background())
	if result.Status != model.HealthStatusUnhealthy {
		t.Errorf("Status = %s, want unhealthy", result.Status)
	}
}

func TestTCPChecker_ContextCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	checker := NewTCPChecker(model.HealthCheckConfig{
		Type:    model.HealthCheckTCP,
		Target:  "192.0.2.1:80", // non-routable address
		Timeout: 30 * time.Second,
	})

	result := checker.Check(ctx)
	if result.Status != model.HealthStatusUnhealthy {
		t.Errorf("Status = %s, want unhealthy", result.Status)
	}
}

// --- NewChecker Factory Tests ---

func TestNewChecker_HTTP(t *testing.T) {
	c, err := NewChecker(model.HealthCheckConfig{Type: model.HealthCheckHTTP})
	if err != nil {
		t.Fatalf("NewChecker: %v", err)
	}
	if c.Type() != model.HealthCheckHTTP {
		t.Errorf("Type = %s, want http", c.Type())
	}
}

func TestNewChecker_TCP(t *testing.T) {
	c, err := NewChecker(model.HealthCheckConfig{Type: model.HealthCheckTCP})
	if err != nil {
		t.Fatalf("NewChecker: %v", err)
	}
	if c.Type() != model.HealthCheckTCP {
		t.Errorf("Type = %s, want tcp", c.Type())
	}
}

func TestNewChecker_Script(t *testing.T) {
	c, err := NewChecker(model.HealthCheckConfig{Type: model.HealthCheckScript})
	if err != nil {
		t.Fatalf("NewChecker: %v", err)
	}
	if c.Type() != model.HealthCheckScript {
		t.Errorf("Type = %s, want script", c.Type())
	}
}

func TestNewChecker_Unknown(t *testing.T) {
	_, err := NewChecker(model.HealthCheckConfig{Type: "unknown"})
	if err == nil {
		t.Error("expected error for unknown type")
	}
}

// --- RingBuffer Tests ---

func TestRingBuffer_Empty(t *testing.T) {
	rb := NewRingBuffer(10)
	all := rb.GetAll()
	if len(all) != 0 {
		t.Errorf("GetAll on empty buffer: got %d items", len(all))
	}
	_, ok := rb.Latest()
	if ok {
		t.Error("Latest should return false on empty buffer")
	}
}

func TestRingBuffer_AddAndGetAll(t *testing.T) {
	rb := NewRingBuffer(5)
	for i := 0; i < 3; i++ {
		rb.Add(model.HealthCheckResult{
			Message: fmt.Sprintf("check-%d", i),
		})
	}

	all := rb.GetAll()
	if len(all) != 3 {
		t.Errorf("expected 3 items, got %d", len(all))
	}
	if all[0].Message != "check-0" {
		t.Errorf("first item = %s, want check-0", all[0].Message)
	}
}

func TestRingBuffer_Wraparound(t *testing.T) {
	rb := NewRingBuffer(3)
	for i := 0; i < 5; i++ {
		rb.Add(model.HealthCheckResult{
			Message: fmt.Sprintf("check-%d", i),
		})
	}

	all := rb.GetAll()
	if len(all) != 3 {
		t.Errorf("expected 3 items (capacity), got %d", len(all))
	}
	// Should be the 3 most recent: check-2, check-3, check-4
	if all[0].Message != "check-2" {
		t.Errorf("oldest after wraparound = %s, want check-2", all[0].Message)
	}
	if all[2].Message != "check-4" {
		t.Errorf("newest = %s, want check-4", all[2].Message)
	}
}

func TestRingBuffer_Latest(t *testing.T) {
	rb := NewRingBuffer(5)
	rb.Add(model.HealthCheckResult{Message: "first"})
	rb.Add(model.HealthCheckResult{Message: "second"})

	latest, ok := rb.Latest()
	if !ok {
		t.Error("Latest should return true")
	}
	if latest.Message != "second" {
		t.Errorf("Latest = %s, want second", latest.Message)
	}
}

func TestRingBuffer_SizeOne(t *testing.T) {
	rb := NewRingBuffer(1)
	rb.Add(model.HealthCheckResult{Message: "a"})
	rb.Add(model.HealthCheckResult{Message: "b"})

	all := rb.GetAll()
	if len(all) != 1 {
		t.Errorf("expected 1 item, got %d", len(all))
	}
	if all[0].Message != "b" {
		t.Errorf("item = %s, want b", all[0].Message)
	}
}
