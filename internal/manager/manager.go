// Package manager orchestrates managed services: state transitions,
// crash-loop detection, dependency-aware start/stop ordering and
// integration with the health, notify and logging subsystems.
package manager

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"path/filepath"

	"github.com/google/uuid"

	"github.com/hilman2/ELNSSM/internal/config"
	"github.com/hilman2/ELNSSM/internal/health"
	"github.com/hilman2/ELNSSM/internal/logging"
	"github.com/hilman2/ELNSSM/internal/model"
	"github.com/hilman2/ELNSSM/internal/notify"
	"github.com/hilman2/ELNSSM/internal/process"
	"github.com/hilman2/ELNSSM/internal/scheduler"
	"github.com/hilman2/ELNSSM/internal/store"
)

// Manager orchestrates all managed services.
type Manager struct {
	services  map[string]*ManagedService
	store     store.Store
	notifier  *notify.Dispatcher
	cfg       *config.Config
	streamer  *logging.Streamer
	scheduler *scheduler.Scheduler
	ctx       context.Context    // long-lived context for process lifecycle
	cancelCtx context.CancelFunc // cancels the lifecycle context
	mu        sync.RWMutex
}

// New creates a new service Manager.
func New(s store.Store, n *notify.Dispatcher, cfg *config.Config, streamer *logging.Streamer) *Manager {
	ctx, cancel := context.WithCancel(context.Background())
	mgr := &Manager{
		services:  make(map[string]*ManagedService),
		store:     s,
		notifier:  n,
		cfg:       cfg,
		streamer:  streamer,
		ctx:       ctx,
		cancelCtx: cancel,
	}
	mgr.scheduler = scheduler.New(mgr, ctx)
	return mgr
}

// Shutdown cancels the manager's lifecycle context. Call this during Guardian shutdown.
func (m *Manager) Shutdown() {
	if m.scheduler != nil {
		m.scheduler.Stop()
	}
	m.cancelCtx()
}

// LoadAll loads all services from the store and config files.
func (m *Manager) LoadAll(ctx context.Context) error {
	// Load from YAML config files
	services, err := config.LoadAllServiceConfigs(m.cfg.ServicesDir())
	if err != nil {
		return fmt.Errorf("loading service configs: %w", err)
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	for _, svc := range services {
		// Restore runtime state
		state, err := m.store.GetServiceState(ctx, svc.ID)
		if err != nil {
			slog.Warn("Could not load runtime state", "service", svc.ID, "error", err)
		}
		if state != nil {
			svc.State = state.State
			svc.RestartCount = state.RestartCount
			svc.LastExitCode = state.LastExitCode
			svc.LastError = state.LastError
		} else {
			svc.State = model.ServiceStateStopped
		}

		ms := NewManagedService(svc)
		m.services[svc.ID] = ms
		slog.Info("Loaded service", "id", svc.ID, "state", svc.State)
	}

	// Load scheduler jobs for all services
	m.scheduler.LoadAll(services)
	m.scheduler.Start()

	return nil
}

// AutoStart starts all services with auto or delayed-auto startup type.
// Services are started in dependency order (topological sort).
func (m *Manager) AutoStart(ctx context.Context) {
	m.mu.RLock()

	// Determine start order via topological sort
	order, err := TopologicalSort(m.services)
	if err != nil {
		slog.Error("Cycle detected in service dependencies, starting in arbitrary order", "error", err)
		// Fall back to arbitrary order
		order = make([]string, 0, len(m.services))
		for id := range m.services {
			order = append(order, id)
		}
	}

	m.mu.RUnlock()

	for _, id := range order {
		m.mu.RLock()
		ms, ok := m.services[id]
		m.mu.RUnlock()
		if !ok {
			continue
		}

		switch ms.Config.StartupType {
		case model.StartupAuto:
			go func(ms *ManagedService) {
				if err := m.startService(ctx, ms); err != nil {
					slog.Error("Failed to auto-start service", "service", ms.Config.ID, "error", err)
				}
			}(ms)
		case model.StartupDelayedAuto:
			go func(ms *ManagedService) {
				delay := ms.Config.StartDelay
				if delay <= 0 {
					delay = 30 * time.Second // default fallback
				}
				slog.Info("Delayed auto-start scheduled", "service", ms.Config.ID, "delay", delay)

				// Abandon the start if the Guardian shuts down while we wait.
				// A plain sleep would start the process afterwards, with
				// nothing left to supervise it.
				if !sleepOrStop(ctx, m.ctx.Done(), delay) {
					slog.Info("Delayed auto-start cancelled", "service", ms.Config.ID)
					return
				}
				if err := m.startService(ctx, ms); err != nil {
					slog.Error("Failed to auto-start service (delayed)", "service", ms.Config.ID, "error", err)
				}
			}(ms)
		}
	}
}

// Add registers a new service.
func (m *Manager) Add(ctx context.Context, svc *model.Service) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.services[svc.ID]; exists {
		return fmt.Errorf("service %q already exists", svc.ID)
	}

	svc.State = model.ServiceStateStopped

	// Save to YAML config
	cfgPath := m.cfg.ServicesDir() + "/" + svc.ID + ".yaml"
	if err := config.SaveServiceConfig(cfgPath, svc); err != nil {
		return fmt.Errorf("saving service config: %w", err)
	}

	// Save to store
	if err := m.store.SaveService(ctx, svc); err != nil {
		return fmt.Errorf("saving service to store: %w", err)
	}

	ms := NewManagedService(svc)
	m.services[svc.ID] = ms

	// Register schedules
	m.scheduler.UpdateService(svc)

	m.emitEvent(ctx, model.EventServiceAdded, svc.ID, "Service added")
	slog.Info("Service added", "id", svc.ID)
	return nil
}

// Remove removes a service. It must be stopped first.
func (m *Manager) Remove(ctx context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	ms, ok := m.services[id]
	if !ok {
		return fmt.Errorf("service %q not found", id)
	}

	if ms.Config.State == model.ServiceStateRunning || ms.Config.State == model.ServiceStateStarting {
		return fmt.Errorf("service %q must be stopped before removal", id)
	}

	// Delete from store
	if err := m.store.DeleteService(ctx, id); err != nil {
		slog.Warn("Could not delete from store", "error", err)
	}

	delete(m.services, id)
	m.scheduler.RemoveService(id)
	m.emitEvent(ctx, model.EventServiceRemoved, id, "Service removed")
	slog.Info("Service removed", "id", id)
	return nil
}

// Start starts a specific service.
func (m *Manager) Start(ctx context.Context, id string) error {
	m.mu.RLock()
	ms, ok := m.services[id]
	m.mu.RUnlock()

	if !ok {
		return fmt.Errorf("service %q not found", id)
	}

	return m.startService(ctx, ms)
}

// Stop stops a specific service.
func (m *Manager) Stop(ctx context.Context, id string) error {
	m.mu.RLock()
	ms, ok := m.services[id]
	m.mu.RUnlock()

	if !ok {
		return fmt.Errorf("service %q not found", id)
	}

	return m.stopService(ctx, ms)
}

// Restart stops and then starts a service.
func (m *Manager) Restart(ctx context.Context, id string) error {
	if err := m.Stop(ctx, id); err != nil {
		return err
	}
	// Small delay to let things settle
	time.Sleep(500 * time.Millisecond)
	return m.Start(ctx, id)
}

// GetScheduler returns the scheduler for API access.
func (m *Manager) GetScheduler() *scheduler.Scheduler {
	return m.scheduler
}

// Get returns a specific managed service.
func (m *Manager) Get(id string) (*ManagedService, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	ms, ok := m.services[id]
	return ms, ok
}

// List returns all service configs with current runtime state.
func (m *Manager) List() []*model.Service {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*model.Service, 0, len(m.services))
	for _, ms := range m.services {
		// m.mu guards the map, not what the entries point at. Copy the config
		// and the two component pointers under the service's own lock, then
		// query the components outside it so their locks never nest inside it.
		ms.mu.Lock()
		svc := *ms.Config
		wrapper := ms.Wrapper
		monitor := ms.ResourceMonitor
		ms.mu.Unlock()

		if wrapper != nil && wrapper.IsRunning() && svc.StartedAt != nil {
			svc.Uptime = time.Since(*svc.StartedAt)
		}
		// Populate live resource metrics
		if monitor != nil {
			sample := monitor.Latest()
			svc.CPUPercent = sample.CPUPercent
			svc.MemoryBytes = sample.MemoryBytes
		}
		result = append(result, &svc)
	}
	return result
}

// StopAll gracefully stops all running services.
func (m *Manager) StopAll(ctx context.Context) error {
	m.mu.RLock()
	services := make([]*ManagedService, 0)
	for _, ms := range m.services {
		if st := ms.state(); st == model.ServiceStateRunning || st == model.ServiceStateStarting {
			services = append(services, ms)
		}
	}
	m.mu.RUnlock()

	var wg sync.WaitGroup
	for _, ms := range services {
		wg.Add(1)
		go func(ms *ManagedService) {
			defer wg.Done()
			// Config.ID never changes after the service is loaded, so it is
			// the one field safe to read here without the lock.
			if err := m.stopService(ctx, ms); err != nil {
				slog.Error("Error stopping service", "service", ms.Config.ID, "error", err)
			}
		}(ms)
	}
	wg.Wait()
	return nil
}

// DetachAll releases all running services without killing them.
// Used during Guardian restart. Returns a map of service ID -> PID.
func (m *Manager) DetachAll() map[string]int {
	m.mu.RLock()
	services := make([]*ManagedService, 0)
	for _, ms := range m.services {
		if ms.state() == model.ServiceStateRunning {
			services = append(services, ms)
		}
	}
	m.mu.RUnlock()

	orphans := make(map[string]int)
	for _, ms := range services {
		ms.mu.Lock()

		// The state check above happened without this lock, so a concurrent
		// stopService may have torn the service down in the meantime.
		if ms.Wrapper == nil {
			ms.mu.Unlock()
			continue
		}

		pid := ms.Wrapper.PID()
		slog.Info("Detaching service for restart", "service", ms.Config.ID, "pid", pid)

		// Close log capture (closes pipes, process keeps running)
		if ms.Capture != nil {
			ms.Capture.Close()
			ms.Capture = nil
		}

		// Stop health runner
		if ms.HealthRunner != nil {
			ms.HealthRunner.Stop()
			ms.HealthRunner = nil
		}

		ms.stopResourceMonitor()
		ms.signalStop()

		// Detach wrapper (releases job object without killing)
		ms.Wrapper.Detach()
		ms.Wrapper = nil

		orphans[ms.Config.ID] = pid
		ms.mu.Unlock()
	}

	return orphans
}

// AdoptOrphans tries to re-adopt processes from a previous Guardian instance.
// Called during startup when a restart state file is found.
func (m *Manager) AdoptOrphans(ctx context.Context, orphans map[string]int) {
	for id, pid := range orphans {
		m.mu.RLock()
		ms, ok := m.services[id]
		m.mu.RUnlock()
		if !ok {
			slog.Warn("Orphan service not found in config, skipping", "service", id, "pid", pid)
			continue
		}

		ms.mu.Lock()
		wrapper := process.NewWrapper(ms.Config)
		if err := wrapper.Adopt(pid); err != nil {
			slog.Warn("Could not adopt orphaned process", "service", id, "pid", pid, "error", err)
			ms.Config.State = model.ServiceStateStopped
			ms.Config.PID = 0
			m.saveRuntimeState(ctx, ms)
			ms.mu.Unlock()
			continue
		}

		ms.Wrapper = wrapper
		ms.Config.State = model.ServiceStateRunning
		ms.Config.PID = pid
		m.saveRuntimeState(ctx, ms)

		slog.Info("Successfully adopted orphaned process", "service", id, "pid", pid)

		// No pipe access for adopted processes (output goes to log files from previous capture).
		// Set up monitoring for crashes.
		ms.stopCh = make(chan struct{})

		// Start monitoring goroutine for the adopted process
		go m.monitorService(ctx, ms)

		// Re-start health checks
		if len(ms.Config.HealthChecks) > 0 {
			ms.HealthRunner = health.NewRunner(ms.Config.ID, ms.Config.HealthChecks, m.store)
			go ms.HealthRunner.Run(ctx)
		}

		// Re-start resource monitor
		if ms.ResourceMonitor == nil {
			interval := ms.Config.ResourceLimits.CheckInterval
			if interval <= 0 {
				interval = 2 * time.Second
			}
			rm := process.NewResourceMonitor(process.ResourceMonitorConfig{
				PID:           pid,
				Limits:        ms.Config.ResourceLimits,
				CheckInterval: interval,
			})
			monCtx, monCancel := context.WithCancel(ctx)
			ms.ResourceMonitor = rm
			ms.resourceCancel = monCancel
			go rm.Run(monCtx)
		}

		ms.mu.Unlock()
	}
}

// Update updates a service configuration.
func (m *Manager) Update(ctx context.Context, id string, svc *model.Service) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	ms, ok := m.services[id]
	if !ok {
		return fmt.Errorf("service %q not found", id)
	}

	// Preserve runtime state
	svc.State = ms.Config.State
	svc.PID = ms.Config.PID
	svc.StartedAt = ms.Config.StartedAt
	svc.RestartCount = ms.Config.RestartCount

	// Preserve encrypted password if not provided in update
	if svc.ServiceAccountPassword == "" {
		svc.ServiceAccountPassword = ms.Config.ServiceAccountPassword
	}

	// Save to YAML
	cfgPath := m.cfg.ServicesDir() + "/" + id + ".yaml"
	if err := config.SaveServiceConfig(cfgPath, svc); err != nil {
		return fmt.Errorf("saving service config: %w", err)
	}

	// Save to store
	if err := m.store.SaveService(ctx, svc); err != nil {
		return fmt.Errorf("saving to store: %w", err)
	}

	ms.Config = svc

	// Update schedules
	m.scheduler.UpdateService(svc)

	m.emitEvent(ctx, model.EventServiceConfigChanged, id, "Service configuration updated")
	return nil
}

func (m *Manager) startService(_ context.Context, ms *ManagedService) error {
	ms.mu.Lock()
	defer ms.mu.Unlock()

	if ms.Config.State == model.ServiceStateRunning {
		return fmt.Errorf("service %q is already running", ms.Config.ID)
	}

	// Auto-set working directory to the executable's directory if not specified
	if ms.Config.WorkingDir == "" {
		ms.Config.WorkingDir = filepath.Dir(ms.Config.Executable)
	}

	ms.Config.State = model.ServiceStateStarting
	slog.Info("Starting service", "id", ms.Config.ID)

	// Use the manager's long-lived context for process lifecycle,
	// NOT the HTTP request context which would cancel when the request ends.
	lifecycleCtx := m.ctx

	// Wait for dependencies before starting
	if len(ms.Config.Dependencies) > 0 {
		ms.mu.Unlock() // Release lock while waiting for dependencies
		if err := m.WaitForDependencies(lifecycleCtx, ms.Config); err != nil {
			ms.mu.Lock()
			ms.Config.State = model.ServiceStateFailed
			ms.Config.LastError = fmt.Sprintf("dependency wait failed: %v", err)
			m.saveRuntimeState(lifecycleCtx, ms)
			return fmt.Errorf("waiting for dependencies: %w", err)
		}
		ms.mu.Lock()
		// Re-check state after re-acquiring lock
		if ms.Config.State == model.ServiceStateRunning {
			return fmt.Errorf("service %q is already running", ms.Config.ID)
		}
	}

	// Execute pre-start hook
	if ms.Config.Hooks.PreStart != nil {
		output, err := process.ExecuteHook(lifecycleCtx, ms.Config.Hooks.PreStart, ms.Config.Environment)
		if err != nil {
			m.emitEvent(lifecycleCtx, model.EventHookFailed, ms.Config.ID,
				fmt.Sprintf("pre_start hook failed: %v (output: %s)", err, truncateStr(output)))
			if ms.Config.Hooks.PreStart.OnFailure == "abort" {
				ms.Config.State = model.ServiceStateFailed
				ms.Config.LastError = fmt.Sprintf("pre_start hook failed: %v", err)
				return fmt.Errorf("pre_start hook failed: %w", err)
			}
		} else {
			m.emitEvent(lifecycleCtx, model.EventHookExecuted, ms.Config.ID, "pre_start hook completed")
		}
	}

	// Create new wrapper
	wrapper := process.NewWrapper(ms.Config)
	if err := wrapper.Start(lifecycleCtx); err != nil {
		ms.Config.State = model.ServiceStateFailed
		ms.Config.LastError = err.Error()
		return fmt.Errorf("starting process: %w", err)
	}

	ms.Wrapper = wrapper
	ms.Config.State = model.ServiceStateRunning
	ms.Config.PID = wrapper.PID()
	now := time.Now()
	ms.Config.StartedAt = &now

	// Start log capture - reads from stdout/stderr pipes to prevent pipe deadlock
	// and writes to log files + streams to WebSocket clients
	logDir := m.cfg.ServiceLogDir(ms.Config.ID)
	capture := logging.NewCapture(ms.Config.ID, logDir, logging.CaptureConfig{
		StdoutFile:    ms.Config.LogConfig.StdoutFile,
		StderrFile:    ms.Config.LogConfig.StderrFile,
		CombineOutput: ms.Config.LogConfig.CombineOutput,
		MaxSizeMB:     int(ms.Config.LogConfig.MaxSize / (1024 * 1024)),
		MaxBackups:    ms.Config.LogConfig.MaxBackups,
		Compress:      ms.Config.LogConfig.Compress,
	}, m.streamer)
	capture.Start(wrapper.Stdout(), wrapper.Stderr())
	ms.Capture = capture

	// Save runtime state
	m.saveRuntimeState(lifecycleCtx, ms)
	m.emitEvent(lifecycleCtx, model.EventServiceStarted, ms.Config.ID, fmt.Sprintf("Service started (PID: %d)", ms.Config.PID))

	// Execute post-start hook
	if ms.Config.Hooks.PostStart != nil {
		go func() {
			output, err := process.ExecuteHook(lifecycleCtx, ms.Config.Hooks.PostStart, ms.Config.Environment)
			if err != nil {
				m.emitEvent(lifecycleCtx, model.EventHookFailed, ms.Config.ID,
					fmt.Sprintf("post_start hook failed: %v (output: %s)", err, truncateStr(output)))
			} else {
				m.emitEvent(lifecycleCtx, model.EventHookExecuted, ms.Config.ID, "post_start hook completed")
			}
		}()
	}

	// Start health checks
	if len(ms.Config.HealthChecks) > 0 {
		ms.HealthRunner = health.NewRunner(ms.Config.ID, ms.Config.HealthChecks, m.store)
		go ms.HealthRunner.Run(lifecycleCtx)
	}

	// Start resource monitor for live metrics (always) + breach detection (if limits set)
	{
		interval := ms.Config.ResourceLimits.CheckInterval
		if interval <= 0 {
			interval = 2 * time.Second
		}
		rm := process.NewResourceMonitor(process.ResourceMonitorConfig{
			PID:           ms.Config.PID,
			Limits:        ms.Config.ResourceLimits,
			CheckInterval: interval,
		})
		monCtx, monCancel := context.WithCancel(lifecycleCtx)
		ms.ResourceMonitor = rm
		ms.resourceCancel = monCancel
		go rm.Run(monCtx)
	}

	// Start monitor goroutine (handles crash detection, restarts, resource breaches)
	ms.stopCh = make(chan struct{})
	go m.monitorService(lifecycleCtx, ms)

	return nil
}

func (m *Manager) stopService(_ context.Context, ms *ManagedService) error {
	ms.mu.Lock()
	defer ms.mu.Unlock()

	if ms.Config.State != model.ServiceStateRunning && ms.Config.State != model.ServiceStateStarting {
		return nil
	}

	ms.Config.State = model.ServiceStateStopping
	slog.Info("Stopping service", "id", ms.Config.ID)

	// Execute pre-stop hook
	if ms.Config.Hooks.PreStop != nil {
		output, err := process.ExecuteHook(context.Background(), ms.Config.Hooks.PreStop, ms.Config.Environment)
		if err != nil {
			m.emitEvent(m.ctx, model.EventHookFailed, ms.Config.ID,
				fmt.Sprintf("pre_stop hook failed: %v (output: %s)", err, truncateStr(output)))
		} else {
			m.emitEvent(m.ctx, model.EventHookExecuted, ms.Config.ID, "pre_stop hook completed")
		}
	}

	// Signal monitor goroutine to stop
	ms.signalStop()

	// Stop health runner
	if ms.HealthRunner != nil {
		ms.HealthRunner.Stop()
		ms.HealthRunner = nil
	}

	ms.stopResourceMonitor()

	// Use a dedicated timeout context for stopping the process,
	// NOT the HTTP request context.
	stopCtx, cancel := context.WithTimeout(context.Background(), ms.Config.StopTimeout+5*time.Second)
	defer cancel()

	// Stop process
	if ms.Wrapper != nil {
		if err := ms.Wrapper.Stop(stopCtx); err != nil {
			slog.Error("Error stopping process", "service", ms.Config.ID, "error", err)
		}
		ms.Wrapper.Close()
		ms.Wrapper = nil
	}

	// Close log capture
	if ms.Capture != nil {
		ms.Capture.Close()
		ms.Capture = nil
	}

	ms.Config.State = model.ServiceStateStopped
	ms.Config.PID = 0
	ms.Config.CPUPercent = 0
	ms.Config.MemoryBytes = 0
	m.saveRuntimeState(m.ctx, ms)
	m.emitEvent(m.ctx, model.EventServiceStopped, ms.Config.ID, "Service stopped")

	// Execute post-stop hook
	if ms.Config.Hooks.PostStop != nil {
		output, err := process.ExecuteHook(context.Background(), ms.Config.Hooks.PostStop, ms.Config.Environment)
		if err != nil {
			m.emitEvent(m.ctx, model.EventHookFailed, ms.Config.ID,
				fmt.Sprintf("post_stop hook failed: %v (output: %s)", err, truncateStr(output)))
		} else {
			m.emitEvent(m.ctx, model.EventHookExecuted, ms.Config.ID, "post_stop hook completed")
		}
	}

	return nil
}

// doRestart performs the actual restart of a service process.
// Caller must NOT hold ms.mu. Returns true if restart succeeded.
func (m *Manager) doRestart(ctx context.Context, ms *ManagedService) bool {
	ms.mu.Lock()
	defer ms.mu.Unlock()

	// Close old capture before restart
	if ms.Capture != nil {
		ms.Capture.Close()
		ms.Capture = nil
	}

	ms.stopResourceMonitor()

	wrapper := process.NewWrapper(ms.Config)
	if err := wrapper.Start(ctx); err != nil {
		slog.Error("Failed to restart service", "service", ms.Config.ID, "error", err)
		ms.Config.State = model.ServiceStateFailed
		ms.Config.LastError = err.Error()
		m.saveRuntimeState(ctx, ms)
		return false
	}

	ms.Wrapper = wrapper
	ms.Config.PID = wrapper.PID()
	now := time.Now()
	ms.Config.StartedAt = &now
	ms.Config.State = model.ServiceStateRunning

	// Restart log capture for new process
	logDir := m.cfg.ServiceLogDir(ms.Config.ID)
	capture := logging.NewCapture(ms.Config.ID, logDir, logging.CaptureConfig{
		StdoutFile:    ms.Config.LogConfig.StdoutFile,
		StderrFile:    ms.Config.LogConfig.StderrFile,
		CombineOutput: ms.Config.LogConfig.CombineOutput,
		MaxSizeMB:     int(ms.Config.LogConfig.MaxSize / (1024 * 1024)),
		MaxBackups:    ms.Config.LogConfig.MaxBackups,
		Compress:      ms.Config.LogConfig.Compress,
	}, m.streamer)
	capture.Start(wrapper.Stdout(), wrapper.Stderr())
	ms.Capture = capture

	// Restart resource monitor for live metrics + breach detection
	{
		interval := ms.Config.ResourceLimits.CheckInterval
		if interval <= 0 {
			interval = 2 * time.Second
		}
		rm := process.NewResourceMonitor(process.ResourceMonitorConfig{
			PID:           ms.Config.PID,
			Limits:        ms.Config.ResourceLimits,
			CheckInterval: interval,
		})
		monCtx, monCancel := context.WithCancel(ctx)
		ms.ResourceMonitor = rm
		ms.resourceCancel = monCancel
		go rm.Run(monCtx)
	}

	m.saveRuntimeState(ctx, ms)
	return true
}

// monitorService watches one service for as long as it is running: it reacts
// to the process exiting, to resource-limit breaches and to failing health
// checks, restarting according to the service's restart policy.
//
// It runs concurrently with every API call, so it touches ms.Config only
// through the accessors on ManagedService. The service ID is the exception:
// it is fixed once the service is loaded and is read directly.
func (m *Manager) monitorService(ctx context.Context, ms *ManagedService) {
	serviceID := ms.Config.ID

	// Take the stop channel once and keep it. A later startService installs a
	// fresh channel for the monitor it starts; watching that one would leave
	// this goroutine waiting for a signal meant for its successor.
	stopCh := ms.stopChannel()

	breachCh, healthFailCh := ms.monitorChannels()

	// Periodic resource sample persistence (every 30s)
	perfTicker := time.NewTicker(30 * time.Second)
	defer perfTicker.Stop()

	for {
		// Read the wrapper once per round and use that pointer for the whole
		// round. stopService and DetachAll clear the field, so re-reading it
		// between the nil check and Wait() is what used to panic here.
		wrapper := ms.wrapper()
		if wrapper == nil {
			return
		}

		select {
		case result := <-wrapper.Wait():
			// Check if we were asked to stop
			select {
			case <-stopCh:
				return
			default:
			}

			if result.Crashed || result.ExitCode != 0 {
				slog.Warn("Service crashed", "service", serviceID, "exit_code", result.ExitCode)
				ms.withConfig(func(svc *model.Service) {
					svc.LastExitCode = result.ExitCode
					if result.Error != nil {
						svc.LastError = result.Error.Error()
					}
				})
				m.emitEvent(ctx, model.EventServiceCrashed, serviceID,
					fmt.Sprintf("Process crashed with exit code %d", result.ExitCode))

				// Evaluate restart policy
				svc := ms.configSnapshot()
				if shouldRestart(&svc) {
					var attempt int
					ms.withConfig(func(svc *model.Service) {
						svc.RestartCount++
						attempt = svc.RestartCount
					})
					delay := calculateRestartDelay(&svc, attempt)
					slog.Info("Restarting service", "service", serviceID, "delay", delay, "attempt", attempt)

					if !sleepOrStop(ctx, stopCh, delay) {
						return
					}
					if !m.doRestart(ctx, ms) {
						return
					}

					m.emitEvent(ctx, model.EventServiceRestarted, serviceID,
						fmt.Sprintf("Service restarted (PID: %d, attempt: %d)", ms.configSnapshot().PID, attempt))

					// Re-acquire channels for new monitors
					breachCh, _ = ms.monitorChannels()
					continue
				}

				slog.Error("Restart limit reached", "service", serviceID)
				ms.withConfig(func(svc *model.Service) {
					svc.State = model.ServiceStateFailed
				})
				m.saveRuntimeStateLocked(ctx, ms)
				m.emitEvent(ctx, model.EventRestartLimitReached, serviceID,
					fmt.Sprintf("Restart limit reached after %d attempts", svc.RestartCount))
				return
			}

			// Clean exit
			if ms.configSnapshot().RestartPolicy.Mode == model.RestartAlways {
				var delay time.Duration
				ms.withConfig(func(svc *model.Service) {
					svc.RestartCount++
					delay = svc.RestartPolicy.Delay
				})
				if delay == 0 {
					delay = 5 * time.Second
				}
				if !sleepOrStop(ctx, stopCh, delay) {
					return
				}
				if !m.doRestart(ctx, ms) {
					return
				}

				// Re-acquire channels
				breachCh, _ = ms.monitorChannels()
				continue
			}

			slog.Info("Service exited cleanly", "service", serviceID)
			ms.withConfig(func(svc *model.Service) {
				svc.State = model.ServiceStateStopped
				svc.PID = 0
				svc.CPUPercent = 0
				svc.MemoryBytes = 0
			})
			m.saveRuntimeStateLocked(ctx, ms)
			return

		case breach := <-breachCh:
			slog.Warn("Resource threshold breached", "service", serviceID, "type", breach.Type, "message", breach.Message)
			m.emitEvent(ctx, model.EventResourceThresholdBreached, serviceID,
				fmt.Sprintf("Resource breach: %s - %s", breach.Type, breach.Message))

			if !m.restartAfterBreach(ctx, ms, stopCh) {
				return
			}
			m.emitEvent(ctx, model.EventServiceRestarted, serviceID,
				fmt.Sprintf("Service restarted after resource breach (PID: %d)", ms.configSnapshot().PID))

			// Re-acquire channels
			breachCh, _ = ms.monitorChannels()
			continue

		case <-healthFailCh: // receives model.HealthCheckResult
			if ms.configSnapshot().RestartPolicy.RestartOnHealthFail {
				slog.Warn("Health check failed, restarting", "service", serviceID)
				m.emitEvent(ctx, model.EventHealthCheckFailed, serviceID, "Health check failed, triggering restart")

				if !m.restartAfterBreach(ctx, ms, stopCh) {
					return
				}
				m.emitEvent(ctx, model.EventServiceRestarted, serviceID,
					fmt.Sprintf("Service restarted after health check failure (PID: %d)", ms.configSnapshot().PID))

				// Re-acquire channels
				breachCh, healthFailCh = ms.monitorChannels()
				continue
			}

		case <-perfTicker.C:
			// Persist resource sample for performance graphs.
			// Errors here are non-fatal: a missed sample is acceptable.
			if monitor := ms.resourceMonitor(); monitor != nil {
				sample := monitor.Latest()
				if err := m.store.AppendResourceSample(ctx, serviceID, model.ResourceSample{
					Timestamp:   sample.Timestamp,
					CPUPercent:  sample.CPUPercent,
					MemoryBytes: sample.MemoryBytes,
				}); err != nil {
					slog.Debug("Failed to persist resource sample", "service", serviceID, "error", err)
				}
			}

		case <-stopCh:
			return
		case <-ctx.Done():
			return
		}
	}
}

// restartAfterBreach tears the current process down and starts a fresh one,
// counting the attempt against the restart policy. It is the path taken when
// the service is still alive but misbehaving, which is why it stops the
// process first, unlike the crash path where it has already exited.
//
// It returns false when the monitor should stop: the service was asked to
// shut down during the delay, or the new process failed to start.
func (m *Manager) restartAfterBreach(ctx context.Context, ms *ManagedService, stopCh <-chan struct{}) bool {
	var delay time.Duration
	var stopTimeout time.Duration
	ms.withConfig(func(svc *model.Service) {
		svc.RestartCount++
		delay = svc.RestartPolicy.Delay
		stopTimeout = svc.StopTimeout
	})
	if delay == 0 {
		delay = 5 * time.Second
	}

	if wrapper := ms.wrapper(); wrapper != nil {
		stopCtx, cancel := context.WithTimeout(context.Background(), stopTimeout+5*time.Second)
		_ = wrapper.Stop(stopCtx)
		wrapper.Close()
		cancel()
	}

	if !sleepOrStop(ctx, stopCh, delay) {
		return false
	}
	return m.doRestart(ctx, ms)
}

// sleepOrStop waits out d and reports whether it completed. It returns false
// as soon as the service is asked to stop or the lifecycle context ends, so a
// Guardian shutdown does not have to wait for a backoff delay that can reach
// the configured max_backoff of several minutes.
func sleepOrStop(ctx context.Context, stopCh <-chan struct{}, d time.Duration) bool {
	timer := time.NewTimer(d)
	defer timer.Stop()

	select {
	case <-timer.C:
		return true
	case <-stopCh:
		return false
	case <-ctx.Done():
		return false
	}
}

// shouldRestart reports whether the restart policy permits another attempt.
// svc is a snapshot so the answer cannot change while the caller acts on it.
func shouldRestart(svc *model.Service) bool {
	rp := svc.RestartPolicy

	switch rp.Mode {
	case model.RestartNever:
		return false
	case model.RestartAlways:
		return svc.RestartCount < rp.MaxRetries || rp.MaxRetries == 0
	case model.RestartOnFailure:
		return svc.RestartCount < rp.MaxRetries || rp.MaxRetries == 0
	default:
		return svc.RestartCount < 10 // safe default
	}
}

// calculateRestartDelay returns the backoff delay before restart number
// attempt, growing by the policy's multiplier and capped at its max_backoff.
// attempt is passed separately from svc because the caller increments the
// counter under the lock and svc is the snapshot taken before that.
func calculateRestartDelay(svc *model.Service, attempt int) time.Duration {
	rp := svc.RestartPolicy
	delay := rp.Delay
	if delay == 0 {
		delay = 5 * time.Second
	}

	multiplier := rp.BackoffMultiplier
	if multiplier <= 0 {
		multiplier = 2.0
	}

	// Exponential backoff
	for i := 1; i < attempt; i++ {
		delay = time.Duration(float64(delay) * multiplier)
		if rp.MaxBackoff > 0 && delay > rp.MaxBackoff {
			delay = rp.MaxBackoff
			break
		}
	}

	return delay
}

// saveRuntimeStateLocked persists the runtime state, taking ms.mu itself.
// Use it from the monitor goroutine, which does not otherwise hold the lock.
func (m *Manager) saveRuntimeStateLocked(ctx context.Context, ms *ManagedService) {
	ms.mu.Lock()
	defer ms.mu.Unlock()
	m.saveRuntimeState(ctx, ms)
}

// saveRuntimeState writes the service's runtime state to the store.
// The caller must hold ms.mu.
func (m *Manager) saveRuntimeState(ctx context.Context, ms *ManagedService) {
	state := &store.ServiceRuntimeState{
		State:        ms.Config.State,
		PID:          ms.Config.PID,
		StartedAt:    ms.Config.StartedAt,
		RestartCount: ms.Config.RestartCount,
		LastExitCode: ms.Config.LastExitCode,
		LastError:    ms.Config.LastError,
	}
	if err := m.store.SaveServiceState(ctx, ms.Config.ID, state); err != nil {
		slog.Error("Failed to save runtime state", "service", ms.Config.ID, "error", err)
	}
}

// hookOutputMaxLen is the maximum length of captured hook output included
// in event messages. Longer output is truncated with an ellipsis.
const hookOutputMaxLen = 200

func truncateStr(s string) string {
	if len(s) <= hookOutputMaxLen {
		return s
	}
	return s[:hookOutputMaxLen] + "..."
}

func (m *Manager) emitEvent(ctx context.Context, eventType model.EventType, serviceID, message string) {
	event := model.Event{
		ID:        uuid.New().String(),
		Type:      eventType,
		ServiceID: serviceID,
		Timestamp: time.Now(),
		Message:   message,
	}
	if err := m.store.AppendEvent(ctx, event); err != nil {
		slog.Error("Failed to store event", "error", err)
	}
	if m.notifier != nil {
		m.notifier.Dispatch(event)
	}
}
