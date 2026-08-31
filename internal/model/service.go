// Package model defines the core domain types used across ELNSSM
// (services, health checks, events and runtime state).
package model

import "time"

// ServiceState represents the lifecycle state of a managed service.
type ServiceState string

const (
	ServiceStateStopped    ServiceState = "stopped"
	ServiceStateStarting   ServiceState = "starting"
	ServiceStateRunning    ServiceState = "running"
	ServiceStateStopping   ServiceState = "stopping"
	ServiceStateFailed     ServiceState = "failed"
	ServiceStateRestarting ServiceState = "restarting"
)

// StartupType determines when a service is automatically started.
type StartupType string

const (
	StartupAuto        StartupType = "auto"
	StartupManual      StartupType = "manual"
	StartupDisabled    StartupType = "disabled"
	StartupDelayedAuto StartupType = "delayed-auto"
)

// StopSignal determines how a service is gracefully stopped.
type StopSignal string

const (
	StopSignalCtrlC     StopSignal = "ctrl_c"
	StopSignalCtrlBreak StopSignal = "ctrl_break"
	StopSignalTerminate StopSignal = "terminate"
	StopSignalWMClose   StopSignal = "wm_close"
)

// ProcessPriority maps to Windows process priority classes.
type ProcessPriority string

const (
	PriorityIdle        ProcessPriority = "idle"
	PriorityBelowNormal ProcessPriority = "below_normal"
	PriorityNormal      ProcessPriority = "normal"
	PriorityAboveNormal ProcessPriority = "above_normal"
	PriorityHigh        ProcessPriority = "high"
	PriorityRealtime    ProcessPriority = "realtime"
)

// RestartMode determines the restart behavior after process exit.
type RestartMode string

const (
	RestartAlways    RestartMode = "always"
	RestartOnFailure RestartMode = "on_failure"
	RestartNever     RestartMode = "never"
)

// RestartPolicyConfig defines how a service should be restarted.
type RestartPolicyConfig struct {
	Mode                RestartMode   `yaml:"mode" json:"mode"`
	Delay               time.Duration `yaml:"delay" json:"delay"`
	MaxRetries          int           `yaml:"max_retries" json:"max_retries"`
	RetryWindow         time.Duration `yaml:"retry_window" json:"retry_window"`
	BackoffMultiplier   float64       `yaml:"backoff_multiplier" json:"backoff_multiplier"`
	MaxBackoff          time.Duration `yaml:"max_backoff" json:"max_backoff"`
	RestartOnHealthFail bool          `yaml:"restart_on_health_fail" json:"restart_on_health_fail"`
	ScheduledRestart    string        `yaml:"scheduled_restart" json:"scheduled_restart"`
}

// HealthCheckType identifies the kind of health check.
type HealthCheckType string

const (
	HealthCheckHTTP   HealthCheckType = "http"
	HealthCheckTCP    HealthCheckType = "tcp"
	HealthCheckScript HealthCheckType = "script"
)

// HealthCheckConfig defines a single health check for a service.
type HealthCheckConfig struct {
	Type         HealthCheckType `yaml:"type" json:"type"`
	Target       string          `yaml:"target" json:"target"`
	Method       string          `yaml:"method,omitempty" json:"method,omitempty"`
	ExpectStatus int             `yaml:"expect_status,omitempty" json:"expect_status,omitempty"`
	ExpectBody   string          `yaml:"expect_body,omitempty" json:"expect_body,omitempty"`
	Send         string          `yaml:"send,omitempty" json:"send,omitempty"`               // TCP: data to send after connect
	ExpectResp   string          `yaml:"expect_resp,omitempty" json:"expect_resp,omitempty"` // TCP: expected substring in response
	ScriptBody   string          `yaml:"script_body,omitempty" json:"script_body,omitempty"` // Script: inline multi-line script
	// Interpreter selects what runs ScriptBody: "cmd" or "powershell".
	// Leaving it empty falls back to guessing from the script text, which
	// gets it wrong often enough to be worth setting; see health.ScriptShell.
	Interpreter string        `yaml:"interpreter,omitempty" json:"interpreter,omitempty"`
	Interval    time.Duration `yaml:"interval" json:"interval"`
	Timeout     time.Duration `yaml:"timeout" json:"timeout"`
	Retries     int           `yaml:"retries" json:"retries"`
	StartDelay  time.Duration `yaml:"start_delay" json:"start_delay"`
}

// LogConfig defines logging settings for a service.
type LogConfig struct {
	StdoutFile    string        `yaml:"stdout_file" json:"stdout_file"`
	StderrFile    string        `yaml:"stderr_file" json:"stderr_file"`
	CombineOutput bool          `yaml:"combine_output" json:"combine_output"`
	MaxSize       int64         `yaml:"max_size" json:"max_size"`
	MaxAge        time.Duration `yaml:"max_age" json:"max_age"`
	MaxBackups    int           `yaml:"max_backups" json:"max_backups"`
	Compress      bool          `yaml:"compress" json:"compress"`
}

// ResourceLimits defines resource usage thresholds that trigger restarts.
type ResourceLimits struct {
	CPUThreshold     float64       `yaml:"cpu_threshold" json:"cpu_threshold"`
	CPUDuration      time.Duration `yaml:"cpu_duration" json:"cpu_duration"`
	MemoryMax        int64         `yaml:"memory_max" json:"memory_max"`
	MemorySpikeRatio float64       `yaml:"memory_spike_ratio" json:"memory_spike_ratio"`
	CheckInterval    time.Duration `yaml:"check_interval" json:"check_interval"`
}

// HasAnyLimit returns true if any resource limit is configured.
func (rl ResourceLimits) HasAnyLimit() bool {
	return rl.CPUThreshold > 0 || rl.MemoryMax > 0 || rl.MemorySpikeRatio > 0
}

// LifecycleHook defines a script or command to run at a specific lifecycle stage.
type LifecycleHook struct {
	Command   string        `yaml:"command,omitempty" json:"command,omitempty"`
	Args      []string      `yaml:"args,omitempty" json:"args,omitempty"`
	Script    string        `yaml:"script,omitempty" json:"script,omitempty"` // Inline script (PS or CMD auto-detected)
	Timeout   time.Duration `yaml:"timeout" json:"timeout"`
	OnFailure string        `yaml:"on_failure" json:"on_failure"` // "abort" or "continue" (default)
}

// LifecycleHooks holds all lifecycle hooks for a service.
type LifecycleHooks struct {
	PreStart  *LifecycleHook `yaml:"pre_start,omitempty" json:"pre_start,omitempty"`
	PostStart *LifecycleHook `yaml:"post_start,omitempty" json:"post_start,omitempty"`
	PreStop   *LifecycleHook `yaml:"pre_stop,omitempty" json:"pre_stop,omitempty"`
	PostStop  *LifecycleHook `yaml:"post_stop,omitempty" json:"post_stop,omitempty"`
}

// ResourceSample is a single point-in-time resource usage measurement.
type ResourceSample struct {
	Timestamp   time.Time `json:"timestamp"`
	CPUPercent  float64   `json:"cpu_percent"`
	MemoryBytes int64     `json:"memory_bytes"`
}

// DependencyType determines how a dependency is considered fulfilled.
type DependencyType string

const (
	DependencyRunning DependencyType = "running" // Service must be in state "running"
	DependencyHealthy DependencyType = "healthy" // Service must pass health checks
)

// ServiceDependency defines a dependency on another service.
type ServiceDependency struct {
	ServiceID string         `yaml:"service" json:"service"`
	Type      DependencyType `yaml:"type" json:"type"`       // running | healthy
	Timeout   time.Duration  `yaml:"timeout" json:"timeout"` // Max wait time
}

// ScheduleAction defines what happens when a schedule triggers.
type ScheduleAction string

const (
	ScheduleStart   ScheduleAction = "start"
	ScheduleStop    ScheduleAction = "stop"
	ScheduleRestart ScheduleAction = "restart"
)

// ScheduleEntry defines a cron-based schedule for a service.
type ScheduleEntry struct {
	Cron   string         `yaml:"cron" json:"cron"`     // Cron expression, e.g. "0 8 * * MON-FRI"
	Action ScheduleAction `yaml:"action" json:"action"` // start, stop, restart
	Name   string         `yaml:"name,omitempty" json:"name,omitempty"`
}

// NotificationOverride allows per-service notification settings.
type NotificationOverride struct {
	Enabled         bool     `yaml:"enabled" json:"enabled"`
	Events          []string `yaml:"events" json:"events"`
	ExtraRecipients []string `yaml:"extra_recipients,omitempty" json:"extra_recipients,omitempty"`
}

// Service is the core domain model for a managed service.
type Service struct {
	ID          string            `yaml:"id" json:"id"`
	Name        string            `yaml:"name" json:"name"`
	DisplayName string            `yaml:"display_name" json:"display_name"`
	Description string            `yaml:"description" json:"description"`
	Executable  string            `yaml:"executable" json:"executable"`
	Arguments   []string          `yaml:"arguments" json:"arguments"`
	WorkingDir  string            `yaml:"working_dir" json:"working_dir"`
	Environment map[string]string `yaml:"environment" json:"environment"`

	StartupType    StartupType   `yaml:"startup_type" json:"startup_type"`
	StartDelay     time.Duration `yaml:"start_delay" json:"start_delay"`
	ServiceAccount string        `yaml:"service_account" json:"service_account"`

	Priority    ProcessPriority `yaml:"priority" json:"priority"`
	StopTimeout time.Duration   `yaml:"stop_timeout" json:"stop_timeout"`
	StopSignal  StopSignal      `yaml:"stop_signal" json:"stop_signal"`

	RestartPolicy  RestartPolicyConfig   `yaml:"restart_policy" json:"restart_policy"`
	HealthChecks   []HealthCheckConfig   `yaml:"health_checks" json:"health_checks"`
	Hooks          LifecycleHooks        `yaml:"hooks,omitempty" json:"hooks,omitempty"`
	Schedules      []ScheduleEntry       `yaml:"schedules,omitempty" json:"schedules,omitempty"`
	Dependencies   []ServiceDependency   `yaml:"dependencies,omitempty" json:"dependencies,omitempty"`
	LogConfig      LogConfig             `yaml:"logging" json:"logging"`
	Notifications  *NotificationOverride `yaml:"notifications,omitempty" json:"notifications,omitempty"`
	ResourceLimits ResourceLimits        `yaml:"resource_limits" json:"resource_limits"`

	ServiceAccountPassword string `yaml:"service_account_password,omitempty" json:"-"` // DPAPI-encrypted, never in API JSON

	// Runtime state (not persisted in YAML, managed by store)
	State        ServiceState  `yaml:"-" json:"state"`
	PID          int           `yaml:"-" json:"pid"`
	Uptime       time.Duration `yaml:"-" json:"uptime"`
	StartedAt    *time.Time    `yaml:"-" json:"started_at,omitempty"`
	RestartCount int           `yaml:"-" json:"restart_count"`
	LastExitCode int           `yaml:"-" json:"last_exit_code"`
	LastError    string        `yaml:"-" json:"last_error,omitempty"`
	CPUPercent   float64       `yaml:"-" json:"cpu_percent"`
	MemoryBytes  int64         `yaml:"-" json:"memory_bytes"`
}
