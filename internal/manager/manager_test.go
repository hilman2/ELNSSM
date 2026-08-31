package manager

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/hilman2/ELNSSM/internal/model"
)

func newTestManager(ids ...string) *Manager {
	m := &Manager{services: make(map[string]*ManagedService)}
	for _, id := range ids {
		m.services[id] = NewManagedService(&model.Service{
			ID:    id,
			State: model.ServiceStateStopped,
		})
	}
	return m
}

// List used to copy *ms.Config while holding only the map lock, which races
// against every write the monitor goroutine makes to the same struct. Run this
// with -race; without the fix it reports a data race on model.Service.
func TestListRacesWithStateWrites(t *testing.T) {
	m := newTestManager("a", "b", "c")

	var wg sync.WaitGroup
	stop := make(chan struct{})

	for _, ms := range m.services {
		wg.Add(1)
		go func(ms *ManagedService) {
			defer wg.Done()
			for i := 0; ; i++ {
				select {
				case <-stop:
					return
				default:
				}
				now := time.Now()
				ms.withConfig(func(svc *model.Service) {
					svc.State = model.ServiceStateRunning
					svc.PID = i
					svc.RestartCount = i
					svc.StartedAt = &now
				})
			}
		}(ms)
	}

	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 2000; i++ {
			for _, svc := range m.List() {
				_ = svc.State
				_ = svc.PID
			}
		}
	}()

	// The readers finish on their own; the writers run until told to stop.
	time.Sleep(50 * time.Millisecond)
	close(stop)
	wg.Wait()
}

// DetachAll signalled the monitor with a non-blocking send, which was dropped
// whenever the monitor was blocked elsewhere. Closing always lands.
func TestSignalStopIsVisibleToALaterReceive(t *testing.T) {
	ms := NewManagedService(&model.Service{ID: "a"})
	stopCh := ms.stopChannel()

	ms.mu.Lock()
	ms.signalStop()
	ms.mu.Unlock()

	// Receive well after the signal was sent, the way the monitor does when it
	// returns from waiting on the process.
	select {
	case <-stopCh:
	case <-time.After(time.Second):
		t.Fatal("stop signal was not delivered to a receiver that arrived late")
	}
}

func TestSignalStopIsIdempotent(t *testing.T) {
	ms := NewManagedService(&model.Service{ID: "a"})

	ms.mu.Lock()
	defer ms.mu.Unlock()

	ms.signalStop()
	ms.signalStop() // stopService and DetachAll can both reach this
}

func TestStopResourceMonitorCancelsGoroutine(t *testing.T) {
	ms := NewManagedService(&model.Service{ID: "a"})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	monCtx, monCancel := context.WithCancel(ctx)
	ms.mu.Lock()
	ms.resourceCancel = monCancel
	ms.mu.Unlock()

	ms.mu.Lock()
	ms.stopResourceMonitor()
	ms.mu.Unlock()

	select {
	case <-monCtx.Done():
	case <-time.After(time.Second):
		t.Fatal("resource monitor context was not cancelled; its goroutine would outlive the service")
	}

	if ms.resourceCancel != nil {
		t.Error("resourceCancel should be cleared after the monitor is stopped")
	}
}

func TestSleepOrStop(t *testing.T) {
	t.Run("completes", func(t *testing.T) {
		if !sleepOrStop(context.Background(), make(chan struct{}), time.Millisecond) {
			t.Error("sleepOrStop = false, want true when the delay elapses")
		}
	})

	t.Run("stops on signal", func(t *testing.T) {
		stopCh := make(chan struct{})
		close(stopCh)
		if sleepOrStop(context.Background(), stopCh, time.Hour) {
			t.Error("sleepOrStop = true, want false when asked to stop")
		}
	})

	t.Run("stops on context", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		if sleepOrStop(ctx, make(chan struct{}), time.Hour) {
			t.Error("sleepOrStop = true, want false when the context ends")
		}
	})
}

func TestShouldRestart(t *testing.T) {
	tests := []struct {
		name    string
		mode    model.RestartMode
		count   int
		retries int
		want    bool
	}{
		{"never", model.RestartNever, 0, 10, false},
		{"always under limit", model.RestartAlways, 3, 10, true},
		{"always at limit", model.RestartAlways, 10, 10, false},
		{"always unlimited", model.RestartAlways, 500, 0, true},
		{"on failure under limit", model.RestartOnFailure, 1, 5, true},
		{"on failure at limit", model.RestartOnFailure, 5, 5, false},
		{"unknown mode falls back", "", 9, 0, true},
		{"unknown mode at fallback limit", "", 10, 0, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := &model.Service{
				RestartCount: tt.count,
				RestartPolicy: model.RestartPolicyConfig{
					Mode:       tt.mode,
					MaxRetries: tt.retries,
				},
			}
			if got := shouldRestart(svc); got != tt.want {
				t.Errorf("shouldRestart = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCalculateRestartDelay(t *testing.T) {
	svc := &model.Service{
		RestartPolicy: model.RestartPolicyConfig{
			Delay:             time.Second,
			BackoffMultiplier: 2.0,
			MaxBackoff:        10 * time.Second,
		},
	}

	tests := []struct {
		attempt int
		want    time.Duration
	}{
		{1, time.Second},
		{2, 2 * time.Second},
		{3, 4 * time.Second},
		{4, 8 * time.Second},
		{5, 10 * time.Second}, // capped at max_backoff
		{9, 10 * time.Second},
	}

	for _, tt := range tests {
		if got := calculateRestartDelay(svc, tt.attempt); got != tt.want {
			t.Errorf("calculateRestartDelay(attempt=%d) = %v, want %v", tt.attempt, got, tt.want)
		}
	}
}

func TestCalculateRestartDelayDefaults(t *testing.T) {
	// An empty policy must still produce a usable delay rather than restarting
	// in a tight loop.
	svc := &model.Service{}
	if got := calculateRestartDelay(svc, 1); got != 5*time.Second {
		t.Errorf("calculateRestartDelay with empty policy = %v, want 5s", got)
	}
}

// wrapper() and the fields it guards are touched from the monitor goroutine and
// from stopService at the same time. Run with -race.
func TestConcurrentAccessorsAreRaceFree(t *testing.T) {
	ms := NewManagedService(&model.Service{ID: "a"})

	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 500; j++ {
				_ = ms.wrapper()
				_ = ms.resourceMonitor()
				_ = ms.state()
				_ = ms.configSnapshot()
				_, _ = ms.monitorChannels()
			}
		}()
	}

	wg.Add(1)
	go func() {
		defer wg.Done()
		for j := 0; j < 500; j++ {
			ms.mu.Lock()
			ms.Wrapper = nil
			ms.ResourceMonitor = nil
			ms.Config.State = model.ServiceStateStopping
			ms.mu.Unlock()
		}
	}()

	wg.Wait()
}
