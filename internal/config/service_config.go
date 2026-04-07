package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/hilman2/ELNSSM/internal/model"
	"gopkg.in/yaml.v3"
)

// ServiceConfigYAML is the YAML-friendly representation of a service configuration.
// Duration fields are strings for human-readable YAML (e.g., "30s", "5m").
type ServiceConfigYAML struct {
	ID          string            `yaml:"id"`
	Name        string            `yaml:"name"`
	DisplayName string            `yaml:"display_name"`
	Description string            `yaml:"description"`
	Executable  string            `yaml:"executable"`
	Arguments   []string          `yaml:"arguments"`
	WorkingDir  string            `yaml:"working_dir"`
	Environment map[string]string `yaml:"environment"`

	StartupType    string `yaml:"startup_type"`
	StartDelay     string `yaml:"start_delay,omitempty"`
	ServiceAccount string `yaml:"service_account"`
	Priority       string `yaml:"priority"`
	StopTimeout    string `yaml:"stop_timeout"`
	StopSignal     string `yaml:"stop_signal"`

	RestartPolicy  RestartPolicyYAML   `yaml:"restart_policy"`
	HealthChecks   []HealthCheckYAML   `yaml:"health_checks"`
	Hooks          *LifecycleHooksYAML `yaml:"hooks,omitempty"`
	Schedules      []ScheduleYAML      `yaml:"schedules,omitempty"`
	Dependencies   []DependencyYAML    `yaml:"dependencies,omitempty"`
	Logging        LoggingYAML         `yaml:"logging"`
	Notifications  *NotificationYAML   `yaml:"notifications,omitempty"`
	ResourceLimits ResourceLimitsYAML  `yaml:"resource_limits,omitempty"`

	ServiceAccountPassword string `yaml:"service_account_password,omitempty"`
}

// HealthCheckYAML is the YAML representation of a health check.
type HealthCheckYAML struct {
	Type         string `yaml:"type"`
	Target       string `yaml:"target"`
	Method       string `yaml:"method,omitempty"`
	ExpectStatus int    `yaml:"expect_status,omitempty"`
	ExpectBody   string `yaml:"expect_body,omitempty"`
	Send         string `yaml:"send,omitempty"`
	ExpectResp   string `yaml:"expect_resp,omitempty"`
	ScriptBody   string `yaml:"script_body,omitempty"`
	Interval     string `yaml:"interval"`
	Timeout      string `yaml:"timeout"`
	Retries      int    `yaml:"retries"`
	StartDelay   string `yaml:"start_delay"`
}

// LoggingYAML is the YAML representation of logging config.
type LoggingYAML struct {
	StdoutFile    string `yaml:"stdout_file"`
	StderrFile    string `yaml:"stderr_file"`
	CombineOutput bool   `yaml:"combine_output"`
	MaxSize       int64  `yaml:"max_size"`
	MaxAge        string `yaml:"max_age"`
	MaxBackups    int    `yaml:"max_backups"`
	Compress      bool   `yaml:"compress"`
}

// NotificationYAML is the YAML representation of per-service notification overrides.
type NotificationYAML struct {
	Enabled         bool     `yaml:"enabled"`
	Events          []string `yaml:"events"`
	ExtraRecipients []string `yaml:"extra_recipients,omitempty"`
}

// DependencyYAML is the YAML representation of a service dependency.
type DependencyYAML struct {
	Service string `yaml:"service"`
	Type    string `yaml:"type"`              // "running" or "healthy"
	Timeout string `yaml:"timeout,omitempty"` // e.g. "120s"
}

// ScheduleYAML is the YAML representation of a schedule entry.
type ScheduleYAML struct {
	Cron   string `yaml:"cron"`
	Action string `yaml:"action"`
	Name   string `yaml:"name,omitempty"`
}

// LifecycleHookYAML is the YAML representation of a lifecycle hook.
type LifecycleHookYAML struct {
	Command   string   `yaml:"command,omitempty"`
	Args      []string `yaml:"args,omitempty"`
	Script    string   `yaml:"script,omitempty"`
	Timeout   string   `yaml:"timeout,omitempty"`
	OnFailure string   `yaml:"on_failure,omitempty"` // "abort" or "continue"
}

// LifecycleHooksYAML is the YAML representation of all lifecycle hooks.
type LifecycleHooksYAML struct {
	PreStart  *LifecycleHookYAML `yaml:"pre_start,omitempty"`
	PostStart *LifecycleHookYAML `yaml:"post_start,omitempty"`
	PreStop   *LifecycleHookYAML `yaml:"pre_stop,omitempty"`
	PostStop  *LifecycleHookYAML `yaml:"post_stop,omitempty"`
}

// ResourceLimitsYAML is the YAML representation of resource limits with human-readable values.
type ResourceLimitsYAML struct {
	CPUThreshold     float64 `yaml:"cpu_threshold,omitempty"`
	CPUDuration      string  `yaml:"cpu_duration,omitempty"`
	MemoryMax        string  `yaml:"memory_max,omitempty"`         // "512MB", "2GB"
	MemorySpikeRatio float64 `yaml:"memory_spike_ratio,omitempty"`
	CheckInterval    string  `yaml:"check_interval,omitempty"`
}

// parseByteSize parses human-readable byte sizes like "512MB", "2GB", "1024KB".
func parseByteSize(s string) (int64, error) {
	s = strings.TrimSpace(s)
	if s == "" || s == "0" {
		return 0, nil
	}

	s = strings.ToUpper(s)
	multiplier := int64(1)
	numStr := s

	switch {
	case strings.HasSuffix(s, "TB"):
		multiplier = 1024 * 1024 * 1024 * 1024
		numStr = strings.TrimSuffix(s, "TB")
	case strings.HasSuffix(s, "GB"):
		multiplier = 1024 * 1024 * 1024
		numStr = strings.TrimSuffix(s, "GB")
	case strings.HasSuffix(s, "MB"):
		multiplier = 1024 * 1024
		numStr = strings.TrimSuffix(s, "MB")
	case strings.HasSuffix(s, "KB"):
		multiplier = 1024
		numStr = strings.TrimSuffix(s, "KB")
	case strings.HasSuffix(s, "B"):
		numStr = strings.TrimSuffix(s, "B")
	}

	numStr = strings.TrimSpace(numStr)
	val, err := strconv.ParseFloat(numStr, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid byte size %q: %w", s, err)
	}
	return int64(val * float64(multiplier)), nil
}

// formatByteSize formats bytes to a human-readable string.
func formatByteSize(b int64) string {
	if b == 0 {
		return ""
	}
	const (
		kb = 1024
		mb = kb * 1024
		gb = mb * 1024
		tb = gb * 1024
	)
	switch {
	case b >= tb:
		return fmt.Sprintf("%.1fTB", float64(b)/float64(tb))
	case b >= gb:
		return fmt.Sprintf("%.1fGB", float64(b)/float64(gb))
	case b >= mb:
		return fmt.Sprintf("%.0fMB", float64(b)/float64(mb))
	case b >= kb:
		return fmt.Sprintf("%.0fKB", float64(b)/float64(kb))
	default:
		return fmt.Sprintf("%dB", b)
	}
}

// LoadServiceConfig reads a service configuration from a YAML file.
func LoadServiceConfig(path string) (*model.Service, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading service config: %w", err)
	}

	var cfg ServiceConfigYAML
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing service config %s: %w", path, err)
	}

	return cfg.ToService()
}

// SaveServiceConfig writes a service configuration to a YAML file.
func SaveServiceConfig(path string, svc *model.Service) error {
	cfg := ServiceToYAML(svc)

	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("creating service config directory: %w", err)
	}

	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshaling service config: %w", err)
	}

	return os.WriteFile(path, data, 0600)
}

// LoadAllServiceConfigs loads all service configs from a directory.
func LoadAllServiceConfigs(dir string) ([]*model.Service, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("reading services directory: %w", err)
	}

	var services []*model.Service
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".yaml") {
			continue
		}
		svc, err := LoadServiceConfig(filepath.Join(dir, entry.Name()))
		if err != nil {
			return nil, err
		}
		services = append(services, svc)
	}
	return services, nil
}

// ToService converts the YAML config to a domain model Service.
func (c *ServiceConfigYAML) ToService() (*model.Service, error) {
	svc := &model.Service{
		ID:             c.ID,
		Name:           c.Name,
		DisplayName:    c.DisplayName,
		Description:    c.Description,
		Executable:     c.Executable,
		Arguments:      c.Arguments,
		WorkingDir:     c.WorkingDir,
		Environment:    c.Environment,
		StartupType:    model.StartupType(c.StartupType),
		ServiceAccount: c.ServiceAccount,
		Priority:       model.ProcessPriority(c.Priority),
		StopSignal:     model.StopSignal(c.StopSignal),
		State:          model.ServiceStateStopped,
	}

	if c.StartDelay != "" {
		d, err := time.ParseDuration(c.StartDelay)
		if err != nil {
			return nil, fmt.Errorf("invalid start_delay %q: %w", c.StartDelay, err)
		}
		svc.StartDelay = d
	}

	if c.StopTimeout != "" {
		d, err := time.ParseDuration(c.StopTimeout)
		if err != nil {
			return nil, fmt.Errorf("invalid stop_timeout %q: %w", c.StopTimeout, err)
		}
		svc.StopTimeout = d
	}

	// Restart policy
	rp := &svc.RestartPolicy
	rp.Mode = model.RestartMode(c.RestartPolicy.Mode)
	rp.MaxRetries = c.RestartPolicy.MaxRetries
	rp.BackoffMultiplier = c.RestartPolicy.BackoffMultiplier
	rp.RestartOnHealthFail = c.RestartPolicy.RestartOnHealthFail
	rp.ScheduledRestart = c.RestartPolicy.ScheduledRestart

	if c.RestartPolicy.Delay != "" {
		d, err := time.ParseDuration(c.RestartPolicy.Delay)
		if err != nil {
			return nil, fmt.Errorf("invalid restart_policy.delay: %w", err)
		}
		rp.Delay = d
	}
	if c.RestartPolicy.RetryWindow != "" {
		d, err := time.ParseDuration(c.RestartPolicy.RetryWindow)
		if err != nil {
			return nil, fmt.Errorf("invalid restart_policy.retry_window: %w", err)
		}
		rp.RetryWindow = d
	}
	if c.RestartPolicy.MaxBackoff != "" {
		d, err := time.ParseDuration(c.RestartPolicy.MaxBackoff)
		if err != nil {
			return nil, fmt.Errorf("invalid restart_policy.max_backoff: %w", err)
		}
		rp.MaxBackoff = d
	}

	// Health checks
	for i, hc := range c.HealthChecks {
		check := model.HealthCheckConfig{
			Type:         model.HealthCheckType(hc.Type),
			Target:       hc.Target,
			Method:       hc.Method,
			ExpectStatus: hc.ExpectStatus,
			ExpectBody:   hc.ExpectBody,
			Send:         hc.Send,
			ExpectResp:   hc.ExpectResp,
			ScriptBody:   hc.ScriptBody,
			Retries:      hc.Retries,
		}
		if hc.Interval != "" {
			d, err := time.ParseDuration(hc.Interval)
			if err != nil {
				return nil, fmt.Errorf("invalid health_checks[%d].interval: %w", i, err)
			}
			check.Interval = d
		}
		if hc.Timeout != "" {
			d, err := time.ParseDuration(hc.Timeout)
			if err != nil {
				return nil, fmt.Errorf("invalid health_checks[%d].timeout: %w", i, err)
			}
			check.Timeout = d
		}
		if hc.StartDelay != "" {
			d, err := time.ParseDuration(hc.StartDelay)
			if err != nil {
				return nil, fmt.Errorf("invalid health_checks[%d].start_delay: %w", i, err)
			}
			check.StartDelay = d
		}
		svc.HealthChecks = append(svc.HealthChecks, check)
	}

	// Lifecycle hooks
	if c.Hooks != nil {
		if c.Hooks.PreStart != nil {
			hook, err := parseHookYAML(c.Hooks.PreStart, "hooks.pre_start")
			if err != nil {
				return nil, err
			}
			svc.Hooks.PreStart = hook
		}
		if c.Hooks.PostStart != nil {
			hook, err := parseHookYAML(c.Hooks.PostStart, "hooks.post_start")
			if err != nil {
				return nil, err
			}
			svc.Hooks.PostStart = hook
		}
		if c.Hooks.PreStop != nil {
			hook, err := parseHookYAML(c.Hooks.PreStop, "hooks.pre_stop")
			if err != nil {
				return nil, err
			}
			svc.Hooks.PreStop = hook
		}
		if c.Hooks.PostStop != nil {
			hook, err := parseHookYAML(c.Hooks.PostStop, "hooks.post_stop")
			if err != nil {
				return nil, err
			}
			svc.Hooks.PostStop = hook
		}
	}

	// Schedules
	for _, s := range c.Schedules {
		svc.Schedules = append(svc.Schedules, model.ScheduleEntry{
			Cron:   s.Cron,
			Action: model.ScheduleAction(s.Action),
			Name:   s.Name,
		})
	}

	// Dependencies
	for i, dep := range c.Dependencies {
		d := model.ServiceDependency{
			ServiceID: dep.Service,
			Type:      model.DependencyType(dep.Type),
		}
		if dep.Timeout != "" {
			t, err := time.ParseDuration(dep.Timeout)
			if err != nil {
				return nil, fmt.Errorf("invalid dependencies[%d].timeout: %w", i, err)
			}
			d.Timeout = t
		}
		svc.Dependencies = append(svc.Dependencies, d)
	}

	// Logging
	svc.LogConfig = model.LogConfig{
		StdoutFile:    c.Logging.StdoutFile,
		StderrFile:    c.Logging.StderrFile,
		CombineOutput: c.Logging.CombineOutput,
		MaxSize:       c.Logging.MaxSize,
		MaxBackups:    c.Logging.MaxBackups,
		Compress:      c.Logging.Compress,
	}
	if c.Logging.MaxAge != "" {
		d, err := time.ParseDuration(c.Logging.MaxAge)
		if err != nil {
			return nil, fmt.Errorf("invalid logging.max_age: %w", err)
		}
		svc.LogConfig.MaxAge = d
	}

	// Notifications
	if c.Notifications != nil {
		svc.Notifications = &model.NotificationOverride{
			Enabled:         c.Notifications.Enabled,
			Events:          c.Notifications.Events,
			ExtraRecipients: c.Notifications.ExtraRecipients,
		}
	}

	// Resource limits
	rl := &svc.ResourceLimits
	rl.CPUThreshold = c.ResourceLimits.CPUThreshold
	rl.MemorySpikeRatio = c.ResourceLimits.MemorySpikeRatio

	if c.ResourceLimits.CPUDuration != "" {
		d, err := time.ParseDuration(c.ResourceLimits.CPUDuration)
		if err != nil {
			return nil, fmt.Errorf("invalid resource_limits.cpu_duration: %w", err)
		}
		rl.CPUDuration = d
	}
	if c.ResourceLimits.MemoryMax != "" {
		b, err := parseByteSize(c.ResourceLimits.MemoryMax)
		if err != nil {
			return nil, fmt.Errorf("invalid resource_limits.memory_max: %w", err)
		}
		rl.MemoryMax = b
	}
	if c.ResourceLimits.CheckInterval != "" {
		d, err := time.ParseDuration(c.ResourceLimits.CheckInterval)
		if err != nil {
			return nil, fmt.Errorf("invalid resource_limits.check_interval: %w", err)
		}
		rl.CheckInterval = d
	}

	// Service account password (DPAPI-encrypted)
	svc.ServiceAccountPassword = c.ServiceAccountPassword

	return svc, nil
}

// ServiceToYAML converts a domain model Service to its YAML representation.
func ServiceToYAML(svc *model.Service) *ServiceConfigYAML {
	cfg := &ServiceConfigYAML{
		ID:             svc.ID,
		Name:           svc.Name,
		DisplayName:    svc.DisplayName,
		Description:    svc.Description,
		Executable:     svc.Executable,
		Arguments:      svc.Arguments,
		WorkingDir:     svc.WorkingDir,
		Environment:    svc.Environment,
		StartupType:    string(svc.StartupType),
		StartDelay:     optDuration(svc.StartDelay),
		ServiceAccount: svc.ServiceAccount,
		Priority:       string(svc.Priority),
		StopTimeout:    svc.StopTimeout.String(),
		StopSignal:     string(svc.StopSignal),
		RestartPolicy: RestartPolicyYAML{
			Mode:                string(svc.RestartPolicy.Mode),
			Delay:               svc.RestartPolicy.Delay.String(),
			MaxRetries:          svc.RestartPolicy.MaxRetries,
			RetryWindow:         svc.RestartPolicy.RetryWindow.String(),
			BackoffMultiplier:   svc.RestartPolicy.BackoffMultiplier,
			MaxBackoff:          svc.RestartPolicy.MaxBackoff.String(),
			RestartOnHealthFail: svc.RestartPolicy.RestartOnHealthFail,
		},
		Logging: LoggingYAML{
			StdoutFile:    svc.LogConfig.StdoutFile,
			StderrFile:    svc.LogConfig.StderrFile,
			CombineOutput: svc.LogConfig.CombineOutput,
			MaxSize:       svc.LogConfig.MaxSize,
			MaxAge:        svc.LogConfig.MaxAge.String(),
			MaxBackups:    svc.LogConfig.MaxBackups,
			Compress:      svc.LogConfig.Compress,
		},
	}

	if svc.RestartPolicy.ScheduledRestart != "" {
		cfg.RestartPolicy.ScheduledRestart = svc.RestartPolicy.ScheduledRestart
	}

	// Lifecycle hooks
	if svc.Hooks.PreStart != nil || svc.Hooks.PostStart != nil || svc.Hooks.PreStop != nil || svc.Hooks.PostStop != nil {
		cfg.Hooks = &LifecycleHooksYAML{
			PreStart:  hookToYAML(svc.Hooks.PreStart),
			PostStart: hookToYAML(svc.Hooks.PostStart),
			PreStop:   hookToYAML(svc.Hooks.PreStop),
			PostStop:  hookToYAML(svc.Hooks.PostStop),
		}
	}

	for _, hc := range svc.HealthChecks {
		cfg.HealthChecks = append(cfg.HealthChecks, HealthCheckYAML{
			Type:         string(hc.Type),
			Target:       hc.Target,
			Method:       hc.Method,
			ExpectStatus: hc.ExpectStatus,
			ExpectBody:   hc.ExpectBody,
			Send:         hc.Send,
			ExpectResp:   hc.ExpectResp,
			ScriptBody:   hc.ScriptBody,
			Interval:     hc.Interval.String(),
			Timeout:      hc.Timeout.String(),
			Retries:      hc.Retries,
			StartDelay:   hc.StartDelay.String(),
		})
	}

	// Schedules
	for _, s := range svc.Schedules {
		cfg.Schedules = append(cfg.Schedules, ScheduleYAML{
			Cron:   s.Cron,
			Action: string(s.Action),
			Name:   s.Name,
		})
	}

	// Dependencies
	for _, dep := range svc.Dependencies {
		cfg.Dependencies = append(cfg.Dependencies, DependencyYAML{
			Service: dep.ServiceID,
			Type:    string(dep.Type),
			Timeout: optDuration(dep.Timeout),
		})
	}

	if svc.Notifications != nil {
		cfg.Notifications = &NotificationYAML{
			Enabled:         svc.Notifications.Enabled,
			Events:          svc.Notifications.Events,
			ExtraRecipients: svc.Notifications.ExtraRecipients,
		}
	}

	// Resource limits
	if svc.ResourceLimits.HasAnyLimit() {
		cfg.ResourceLimits = ResourceLimitsYAML{
			CPUThreshold:     svc.ResourceLimits.CPUThreshold,
			MemorySpikeRatio: svc.ResourceLimits.MemorySpikeRatio,
		}
		if svc.ResourceLimits.CPUDuration > 0 {
			cfg.ResourceLimits.CPUDuration = svc.ResourceLimits.CPUDuration.String()
		}
		if svc.ResourceLimits.MemoryMax > 0 {
			cfg.ResourceLimits.MemoryMax = formatByteSize(svc.ResourceLimits.MemoryMax)
		}
		if svc.ResourceLimits.CheckInterval > 0 {
			cfg.ResourceLimits.CheckInterval = svc.ResourceLimits.CheckInterval.String()
		}
	}

	// Service account password (DPAPI-encrypted blob)
	cfg.ServiceAccountPassword = svc.ServiceAccountPassword

	return cfg
}

// parseHookYAML converts a YAML hook to a model hook.
func parseHookYAML(h *LifecycleHookYAML, field string) (*model.LifecycleHook, error) {
	hook := &model.LifecycleHook{
		Command:   h.Command,
		Args:      h.Args,
		Script:    h.Script,
		OnFailure: h.OnFailure,
	}
	if hook.OnFailure == "" {
		hook.OnFailure = "continue"
	}
	if h.Timeout != "" {
		d, err := time.ParseDuration(h.Timeout)
		if err != nil {
			return nil, fmt.Errorf("invalid %s.timeout: %w", field, err)
		}
		hook.Timeout = d
	}
	return hook, nil
}

// hookToYAML converts a model hook to its YAML representation.
func hookToYAML(h *model.LifecycleHook) *LifecycleHookYAML {
	if h == nil {
		return nil
	}
	return &LifecycleHookYAML{
		Command:   h.Command,
		Args:      h.Args,
		Script:    h.Script,
		Timeout:   optDuration(h.Timeout),
		OnFailure: h.OnFailure,
	}
}

// optDuration returns the duration string or empty if zero (for omitempty YAML fields).
func optDuration(d time.Duration) string {
	if d <= 0 {
		return ""
	}
	return d.String()
}
