package model

import (
	"encoding/json"
	"testing"
	"time"
)

// --- ServiceState Tests ---

func TestServiceState_Values(t *testing.T) {
	states := []ServiceState{
		ServiceStateStopped,
		ServiceStateStarting,
		ServiceStateRunning,
		ServiceStateStopping,
		ServiceStateFailed,
		ServiceStateRestarting,
	}
	expected := []string{"stopped", "starting", "running", "stopping", "failed", "restarting"}

	for i, s := range states {
		if string(s) != expected[i] {
			t.Errorf("state[%d] = %q, want %q", i, s, expected[i])
		}
	}
}

// --- StartupType Tests ---

func TestStartupType_Values(t *testing.T) {
	tests := []struct {
		got  StartupType
		want string
	}{
		{StartupAuto, "auto"},
		{StartupManual, "manual"},
		{StartupDisabled, "disabled"},
		{StartupDelayedAuto, "delayed-auto"},
	}
	for _, tt := range tests {
		if string(tt.got) != tt.want {
			t.Errorf("StartupType = %q, want %q", tt.got, tt.want)
		}
	}
}

// --- StopSignal Tests ---

func TestStopSignal_Values(t *testing.T) {
	tests := []struct {
		got  StopSignal
		want string
	}{
		{StopSignalCtrlC, "ctrl_c"},
		{StopSignalCtrlBreak, "ctrl_break"},
		{StopSignalTerminate, "terminate"},
		{StopSignalWMClose, "wm_close"},
	}
	for _, tt := range tests {
		if string(tt.got) != tt.want {
			t.Errorf("StopSignal = %q, want %q", tt.got, tt.want)
		}
	}
}

// --- ProcessPriority Tests ---

func TestProcessPriority_Values(t *testing.T) {
	tests := []struct {
		got  ProcessPriority
		want string
	}{
		{PriorityIdle, "idle"},
		{PriorityBelowNormal, "below_normal"},
		{PriorityNormal, "normal"},
		{PriorityAboveNormal, "above_normal"},
		{PriorityHigh, "high"},
		{PriorityRealtime, "realtime"},
	}
	for _, tt := range tests {
		if string(tt.got) != tt.want {
			t.Errorf("ProcessPriority = %q, want %q", tt.got, tt.want)
		}
	}
}

// --- RestartMode Tests ---

func TestRestartMode_Values(t *testing.T) {
	tests := []struct {
		got  RestartMode
		want string
	}{
		{RestartAlways, "always"},
		{RestartOnFailure, "on_failure"},
		{RestartNever, "never"},
	}
	for _, tt := range tests {
		if string(tt.got) != tt.want {
			t.Errorf("RestartMode = %q, want %q", tt.got, tt.want)
		}
	}
}

// --- HealthCheckType Tests ---

func TestHealthCheckType_Values(t *testing.T) {
	tests := []struct {
		got  HealthCheckType
		want string
	}{
		{HealthCheckHTTP, "http"},
		{HealthCheckTCP, "tcp"},
		{HealthCheckScript, "script"},
	}
	for _, tt := range tests {
		if string(tt.got) != tt.want {
			t.Errorf("HealthCheckType = %q, want %q", tt.got, tt.want)
		}
	}
}

// --- Service JSON Tests ---

func TestService_JSONSerialization(t *testing.T) {
	now := time.Now().Truncate(time.Millisecond)
	svc := Service{
		ID:          "my-app",
		Name:        "My App",
		DisplayName: "My Application",
		Executable:  `C:\apps\myapp.exe`,
		Arguments:   []string{"--port", "8080"},
		Environment: map[string]string{"NODE_ENV": "production"},
		StartupType: StartupAuto,
		Priority:    PriorityNormal,
		StopTimeout: 30 * time.Second,
		StopSignal:  StopSignalCtrlC,
		State:       ServiceStateRunning,
		PID:         12345,
		StartedAt:   &now,
	}

	data, err := json.Marshal(svc)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	var decoded Service
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if decoded.ID != svc.ID {
		t.Errorf("ID = %q, want %q", decoded.ID, svc.ID)
	}
	if decoded.Executable != svc.Executable {
		t.Errorf("Executable = %q, want %q", decoded.Executable, svc.Executable)
	}
	if len(decoded.Arguments) != 2 {
		t.Errorf("Arguments len = %d, want 2", len(decoded.Arguments))
	}
	if decoded.Environment["NODE_ENV"] != "production" {
		t.Errorf("NODE_ENV = %q", decoded.Environment["NODE_ENV"])
	}
	if decoded.StartupType != StartupAuto {
		t.Errorf("StartupType = %q, want auto", decoded.StartupType)
	}
}

func TestService_ZeroValue(t *testing.T) {
	var svc Service
	if svc.ID != "" {
		t.Error("zero ID should be empty")
	}
	if svc.State != "" {
		t.Error("zero State should be empty")
	}
	if svc.PID != 0 {
		t.Error("zero PID should be 0")
	}
	if svc.StartedAt != nil {
		t.Error("zero StartedAt should be nil")
	}
	if svc.Notifications != nil {
		t.Error("zero Notifications should be nil")
	}
}

// --- Event Tests ---

func TestEvent_JSONSerialization(t *testing.T) {
	event := Event{
		ID:        "evt-123",
		Type:      EventServiceCrashed,
		ServiceID: "my-app",
		Timestamp: time.Now().Truncate(time.Millisecond),
		Message:   "process exited with code 1",
		Data:      map[string]any{"exit_code": float64(1)},
	}

	data, err := json.Marshal(event)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	var decoded Event
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if decoded.Type != EventServiceCrashed {
		t.Errorf("Type = %q, want service.crashed", decoded.Type)
	}
	if decoded.ServiceID != "my-app" {
		t.Errorf("ServiceID = %q", decoded.ServiceID)
	}
	if decoded.Data["exit_code"] != float64(1) {
		t.Errorf("Data[exit_code] = %v", decoded.Data["exit_code"])
	}
}

func TestEventType_AllValues(t *testing.T) {
	types := []EventType{
		EventServiceStarted,
		EventServiceStopped,
		EventServiceCrashed,
		EventServiceRestarted,
		EventServiceAdded,
		EventServiceRemoved,
		EventServiceConfigChanged,
		EventHealthCheckFailed,
		EventHealthCheckRecovered,
		EventRestartLimitReached,
	}
	if len(types) != 10 {
		t.Errorf("expected 10 event types, got %d", len(types))
	}
	// Verify all are non-empty
	for i, et := range types {
		if et == "" {
			t.Errorf("event type %d is empty", i)
		}
	}
}

// --- HealthStatus Tests ---

func TestHealthStatus_Values(t *testing.T) {
	tests := []struct {
		got  HealthStatus
		want string
	}{
		{HealthStatusHealthy, "healthy"},
		{HealthStatusUnhealthy, "unhealthy"},
		{HealthStatusUnknown, "unknown"},
	}
	for _, tt := range tests {
		if string(tt.got) != tt.want {
			t.Errorf("HealthStatus = %q, want %q", tt.got, tt.want)
		}
	}
}

func TestHealthCheckResult_JSONSerialization(t *testing.T) {
	result := HealthCheckResult{
		CheckType:  HealthCheckHTTP,
		Status:     HealthStatusHealthy,
		Timestamp:  time.Now().Truncate(time.Millisecond),
		Duration:   150 * time.Millisecond,
		Message:    "OK",
		StatusCode: 200,
	}

	data, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	var decoded HealthCheckResult
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if decoded.CheckType != HealthCheckHTTP {
		t.Errorf("CheckType = %q", decoded.CheckType)
	}
	if decoded.Status != HealthStatusHealthy {
		t.Errorf("Status = %q", decoded.Status)
	}
	if decoded.StatusCode != 200 {
		t.Errorf("StatusCode = %d", decoded.StatusCode)
	}
}

// --- RestartPolicyConfig Tests ---

func TestRestartPolicyConfig_Defaults(t *testing.T) {
	var rp RestartPolicyConfig
	if rp.Mode != "" {
		t.Error("zero Mode should be empty")
	}
	if rp.MaxRetries != 0 {
		t.Error("zero MaxRetries should be 0")
	}
	if rp.BackoffMultiplier != 0 {
		t.Error("zero BackoffMultiplier should be 0")
	}
	if rp.RestartOnHealthFail {
		t.Error("zero RestartOnHealthFail should be false")
	}
}

// --- HealthCheckConfig Tests ---

func TestHealthCheckConfig_Defaults(t *testing.T) {
	var hc HealthCheckConfig
	if hc.Type != "" {
		t.Error("zero Type should be empty")
	}
	if hc.ExpectStatus != 0 {
		t.Error("zero ExpectStatus should be 0")
	}
	if hc.Retries != 0 {
		t.Error("zero Retries should be 0")
	}
}
