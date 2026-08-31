package manager

import (
	"context"
	"sync"

	"github.com/hilman2/ELNSSM/internal/health"
	"github.com/hilman2/ELNSSM/internal/logging"
	"github.com/hilman2/ELNSSM/internal/model"
	"github.com/hilman2/ELNSSM/internal/process"
)

// ManagedService bundles a service config with its runtime components.
//
// Every field including Config is guarded by mu. Config is shared with the API
// layer, which serves it while the monitor goroutine writes State, PID and
// RestartCount, so reading it without the lock is a data race even though the
// manager's own map lock is held.
type ManagedService struct {
	Config          *model.Service
	Wrapper         *process.Wrapper
	HealthRunner    *health.Runner
	Capture         *logging.Capture
	ResourceMonitor *process.ResourceMonitor

	// resourceCancel stops the goroutine behind ResourceMonitor. Clearing the
	// ResourceMonitor field only drops the reference; without calling this the
	// goroutine keeps polling a dead PID until the Guardian exits, and Windows
	// hands that PID to an unrelated process soon enough.
	resourceCancel context.CancelFunc

	stopCh chan struct{} // closed to signal the monitor goroutine to stop
	mu     sync.Mutex
}

// NewManagedService creates a new ManagedService from a service config.
func NewManagedService(svc *model.Service) *ManagedService {
	return &ManagedService{
		Config: svc,
		stopCh: make(chan struct{}),
	}
}

// wrapper returns the process wrapper, or nil when no process is attached.
//
// Callers take the pointer once and then work with what they got. Re-reading
// ms.Wrapper after a nil check is what allowed stopService and DetachAll to
// clear the field in between, leaving the monitor goroutine to dereference nil
// and take the whole Guardian down with it.
func (ms *ManagedService) wrapper() *process.Wrapper {
	ms.mu.Lock()
	defer ms.mu.Unlock()
	return ms.Wrapper
}

// stopChannel returns the channel that signals this service's monitor to stop.
//
// The monitor reads it once at start-up and keeps that channel for its whole
// life. A later start installs a fresh channel for its own monitor, and an old
// monitor watching the new channel would never see its own stop signal.
func (ms *ManagedService) stopChannel() <-chan struct{} {
	ms.mu.Lock()
	defer ms.mu.Unlock()
	return ms.stopCh
}

// stopResourceMonitor cancels the resource monitor goroutine and drops the
// reference. Safe to call when none is running. The caller must hold mu.
func (ms *ManagedService) stopResourceMonitor() {
	if ms.resourceCancel != nil {
		ms.resourceCancel()
		ms.resourceCancel = nil
	}
	ms.ResourceMonitor = nil
}

// signalStop tells the monitor goroutine to stop, and does nothing if it has
// already been told. The caller must hold mu, which is what makes the
// closed-check and the close atomic against each other.
//
// Closing rather than sending is deliberate. The monitor spends most of its
// time blocked on the process exit channel, so a non-blocking send finds no
// receiver and is silently dropped; the goroutine then outlives the service it
// was watching and can restart a process the Guardian has already handed on.
func (ms *ManagedService) signalStop() {
	select {
	case <-ms.stopCh:
		// Already closed.
	default:
		close(ms.stopCh)
	}
}

// withConfig runs fn with mu held, so it can read or write Config safely.
// fn must not call back into the Manager, which would risk taking mu twice.
func (ms *ManagedService) withConfig(fn func(svc *model.Service)) {
	ms.mu.Lock()
	defer ms.mu.Unlock()
	fn(ms.Config)
}

// configSnapshot returns a copy of the service config taken under the lock.
func (ms *ManagedService) configSnapshot() model.Service {
	ms.mu.Lock()
	defer ms.mu.Unlock()
	return *ms.Config
}

// state returns the current lifecycle state.
func (ms *ManagedService) state() model.ServiceState {
	ms.mu.Lock()
	defer ms.mu.Unlock()
	return ms.Config.State
}

// resourceMonitor returns the resource monitor, or nil when none is running.
// A restart swaps it out, so read it fresh rather than holding on to one.
func (ms *ManagedService) resourceMonitor() *process.ResourceMonitor {
	ms.mu.Lock()
	defer ms.mu.Unlock()
	return ms.ResourceMonitor
}

// monitorChannels returns the channels the monitor goroutine selects on: the
// resource-breach channel and the health-failure channel. Either is nil when
// the corresponding component is not running, and a nil channel blocks
// forever, which is exactly what a select needs for a branch that cannot fire.
func (ms *ManagedService) monitorChannels() (<-chan process.ResourceBreach, <-chan model.HealthCheckResult) {
	ms.mu.Lock()
	defer ms.mu.Unlock()

	var breachCh <-chan process.ResourceBreach
	if ms.ResourceMonitor != nil {
		breachCh = ms.ResourceMonitor.BreachCh()
	}

	var healthCh <-chan model.HealthCheckResult
	if ms.HealthRunner != nil {
		healthCh = ms.HealthRunner.FailureCh()
	}

	return breachCh, healthCh
}
