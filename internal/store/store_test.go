package store

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/hilman2/ELNSSM/internal/model"
)

func newTestStore(t *testing.T) *BoltStore {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "test.db")
	s, err := NewBoltStore(path)
	if err != nil {
		t.Fatalf("NewBoltStore: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func TestNewBoltStore(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.db")
	s, err := NewBoltStore(path)
	if err != nil {
		t.Fatalf("NewBoltStore: %v", err)
	}
	defer s.Close()

	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Error("database file not created")
	}
}

func TestNewBoltStore_CreatesDir(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "sub", "dir")
	path := filepath.Join(dir, "test.db")
	s, err := NewBoltStore(path)
	if err != nil {
		t.Fatalf("NewBoltStore: %v", err)
	}
	defer s.Close()

	if _, err := os.Stat(dir); os.IsNotExist(err) {
		t.Error("parent directory not created")
	}
}

func TestSaveAndGetService(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	svc := &model.Service{
		ID:          "test-svc",
		Name:        "Test Service",
		Executable:  "C:\\test.exe",
		Arguments:   []string{"--port", "8080"},
		StopTimeout: 30 * time.Second,
		StartupType: model.StartupAuto,
	}

	if err := s.SaveService(ctx, svc); err != nil {
		t.Fatalf("SaveService: %v", err)
	}

	got, err := s.GetService(ctx, "test-svc")
	if err != nil {
		t.Fatalf("GetService: %v", err)
	}

	if got.ID != svc.ID {
		t.Errorf("ID = %q, want %q", got.ID, svc.ID)
	}
	if got.Name != svc.Name {
		t.Errorf("Name = %q, want %q", got.Name, svc.Name)
	}
	if got.Executable != svc.Executable {
		t.Errorf("Executable = %q, want %q", got.Executable, svc.Executable)
	}
	if len(got.Arguments) != 2 {
		t.Errorf("Arguments len = %d, want 2", len(got.Arguments))
	}
}

func TestGetService_NotFound(t *testing.T) {
	s := newTestStore(t)
	_, err := s.GetService(context.Background(), "nonexistent")
	if err == nil {
		t.Error("expected error for non-existent service")
	}
}

func TestListServices_Empty(t *testing.T) {
	s := newTestStore(t)
	services, err := s.ListServices(context.Background())
	if err != nil {
		t.Fatalf("ListServices: %v", err)
	}
	if len(services) != 0 {
		t.Errorf("expected 0 services, got %d", len(services))
	}
}

func TestListServices_Multiple(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	for _, id := range []string{"svc-a", "svc-b", "svc-c"} {
		s.SaveService(ctx, &model.Service{ID: id, Name: id, Executable: "test.exe"})
	}

	services, err := s.ListServices(ctx)
	if err != nil {
		t.Fatalf("ListServices: %v", err)
	}
	if len(services) != 3 {
		t.Errorf("expected 3 services, got %d", len(services))
	}
}

func TestDeleteService(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	s.SaveService(ctx, &model.Service{ID: "del-me", Executable: "test.exe"})
	if err := s.DeleteService(ctx, "del-me"); err != nil {
		t.Fatalf("DeleteService: %v", err)
	}

	_, err := s.GetService(ctx, "del-me")
	if err == nil {
		t.Error("expected error after deletion")
	}
}

func TestSaveAndGetServiceState(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	now := time.Now()

	state := &ServiceRuntimeState{
		State:        model.ServiceStateRunning,
		PID:          12345,
		StartedAt:    &now,
		RestartCount: 3,
		LastExitCode: 1,
		LastError:    "crashed",
	}

	if err := s.SaveServiceState(ctx, "test-svc", state); err != nil {
		t.Fatalf("SaveServiceState: %v", err)
	}

	got, err := s.GetServiceState(ctx, "test-svc")
	if err != nil {
		t.Fatalf("GetServiceState: %v", err)
	}

	if got.State != model.ServiceStateRunning {
		t.Errorf("State = %q, want running", got.State)
	}
	if got.PID != 12345 {
		t.Errorf("PID = %d, want 12345", got.PID)
	}
	if got.RestartCount != 3 {
		t.Errorf("RestartCount = %d, want 3", got.RestartCount)
	}
}

func TestGetServiceState_NotFound(t *testing.T) {
	s := newTestStore(t)
	state, err := s.GetServiceState(context.Background(), "nonexistent")
	if err != nil {
		t.Fatalf("GetServiceState: %v", err)
	}
	// Should return zero-value state
	if state.PID != 0 {
		t.Errorf("PID = %d, want 0", state.PID)
	}
}

func TestAppendAndListEvents(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	events := []model.Event{
		{ID: "1", Type: model.EventServiceStarted, ServiceID: "svc-a", Timestamp: time.Now().Add(-2 * time.Minute), Message: "started"},
		{ID: "2", Type: model.EventServiceCrashed, ServiceID: "svc-a", Timestamp: time.Now().Add(-1 * time.Minute), Message: "crashed"},
		{ID: "3", Type: model.EventServiceStarted, ServiceID: "svc-b", Timestamp: time.Now(), Message: "started"},
	}

	for _, e := range events {
		if err := s.AppendEvent(ctx, e); err != nil {
			t.Fatalf("AppendEvent: %v", err)
		}
	}

	// List all
	result, err := s.ListEvents(ctx, EventFilter{Limit: 10})
	if err != nil {
		t.Fatalf("ListEvents: %v", err)
	}
	if len(result) != 3 {
		t.Errorf("expected 3 events, got %d", len(result))
	}
	// Newest first
	if result[0].ID != "3" {
		t.Errorf("first event should be newest, got ID=%s", result[0].ID)
	}
}

func TestListEvents_FilterByServiceID(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	s.AppendEvent(ctx, model.Event{ID: "1", ServiceID: "svc-a", Timestamp: time.Now().Add(-1 * time.Minute), Type: model.EventServiceStarted})
	s.AppendEvent(ctx, model.Event{ID: "2", ServiceID: "svc-b", Timestamp: time.Now(), Type: model.EventServiceStarted})

	sid := "svc-a"
	result, err := s.ListEvents(ctx, EventFilter{ServiceID: &sid, Limit: 10})
	if err != nil {
		t.Fatalf("ListEvents: %v", err)
	}
	if len(result) != 1 {
		t.Errorf("expected 1 event for svc-a, got %d", len(result))
	}
}

func TestListEvents_Limit(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	for i := 0; i < 10; i++ {
		s.AppendEvent(ctx, model.Event{
			ID:        string(rune('a' + i)),
			ServiceID: "svc",
			Timestamp: time.Now().Add(time.Duration(i) * time.Second),
			Type:      model.EventServiceStarted,
		})
	}

	result, err := s.ListEvents(ctx, EventFilter{Limit: 3})
	if err != nil {
		t.Fatalf("ListEvents: %v", err)
	}
	if len(result) != 3 {
		t.Errorf("expected 3 events, got %d", len(result))
	}
}

func TestAppendAndGetHealthResult(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	results := []model.HealthCheckResult{
		{CheckType: model.HealthCheckHTTP, Status: model.HealthStatusHealthy, Timestamp: time.Now().Add(-2 * time.Minute), Duration: 50 * time.Millisecond},
		{CheckType: model.HealthCheckHTTP, Status: model.HealthStatusUnhealthy, Timestamp: time.Now().Add(-1 * time.Minute), Duration: 5 * time.Second},
		{CheckType: model.HealthCheckHTTP, Status: model.HealthStatusHealthy, Timestamp: time.Now(), Duration: 30 * time.Millisecond},
	}

	for _, r := range results {
		if err := s.AppendHealthResult(ctx, "svc-a", r); err != nil {
			t.Fatalf("AppendHealthResult: %v", err)
		}
	}

	history, err := s.GetHealthHistory(ctx, "svc-a", 10)
	if err != nil {
		t.Fatalf("GetHealthHistory: %v", err)
	}
	if len(history) != 3 {
		t.Errorf("expected 3 results, got %d", len(history))
	}
	// Chronological order
	if history[0].Status != model.HealthStatusHealthy {
		t.Error("first result should be oldest (healthy)")
	}
}

func TestGetHealthHistory_Empty(t *testing.T) {
	s := newTestStore(t)
	history, err := s.GetHealthHistory(context.Background(), "nonexistent", 10)
	if err != nil {
		t.Fatalf("GetHealthHistory: %v", err)
	}
	if len(history) != 0 {
		t.Errorf("expected 0 results, got %d", len(history))
	}
}

func TestGetHealthHistory_Limit(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	for i := 0; i < 20; i++ {
		s.AppendHealthResult(ctx, "svc", model.HealthCheckResult{
			CheckType: model.HealthCheckHTTP,
			Status:    model.HealthStatusHealthy,
			Timestamp: time.Now().Add(time.Duration(i) * time.Second),
		})
	}

	history, err := s.GetHealthHistory(ctx, "svc", 5)
	if err != nil {
		t.Fatalf("GetHealthHistory: %v", err)
	}
	if len(history) != 5 {
		t.Errorf("expected 5 results, got %d", len(history))
	}
}

func TestRunMigrations_Fresh(t *testing.T) {
	s := newTestStore(t)
	if err := s.RunMigrations(); err != nil {
		t.Fatalf("RunMigrations: %v", err)
	}
	// Second run should be no-op
	if err := s.RunMigrations(); err != nil {
		t.Fatalf("RunMigrations (second): %v", err)
	}
}
