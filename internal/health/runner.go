package health

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/hilman2/ELNSSM/internal/model"
	"github.com/hilman2/ELNSSM/internal/store"
)

// Runner manages all health checks for a single service.
type Runner struct {
	serviceID   string
	checks      []model.HealthCheckConfig
	store       store.Store
	failureCh   chan model.HealthCheckResult
	cancel      context.CancelFunc
	wg          sync.WaitGroup
	lastHealthy bool
	healthMu    sync.RWMutex
}

// NewRunner creates a new health check runner.
func NewRunner(serviceID string, checks []model.HealthCheckConfig, s store.Store) *Runner {
	return &Runner{
		serviceID: serviceID,
		checks:    checks,
		store:     s,
		failureCh: make(chan model.HealthCheckResult, 10),
	}
}

// FailureCh returns a channel that receives health check failures.
func (r *Runner) FailureCh() <-chan model.HealthCheckResult {
	return r.failureCh
}

// IsHealthy returns true if the last health check result was healthy.
func (r *Runner) IsHealthy() bool {
	r.healthMu.RLock()
	defer r.healthMu.RUnlock()
	return r.lastHealthy
}

// Run starts all health check loops. Blocks until stopped.
func (r *Runner) Run(ctx context.Context) {
	ctx, r.cancel = context.WithCancel(ctx)

	for _, checkCfg := range r.checks {
		checker, err := NewChecker(checkCfg)
		if err != nil {
			slog.Error("Failed to create health checker", "service", r.serviceID, "error", err)
			continue
		}

		r.wg.Add(1)
		go r.runCheck(ctx, checker, checkCfg)
	}

	r.wg.Wait()
}

// Stop cancels all health check loops.
func (r *Runner) Stop() {
	if r.cancel != nil {
		r.cancel()
	}
	r.wg.Wait()
}

func (r *Runner) runCheck(ctx context.Context, checker Checker, cfg model.HealthCheckConfig) {
	defer r.wg.Done()

	// Wait for start delay
	if cfg.StartDelay > 0 {
		select {
		case <-time.After(cfg.StartDelay):
		case <-ctx.Done():
			return
		}
	}

	interval := cfg.Interval
	if interval == 0 {
		interval = 30 * time.Second
	}

	consecutiveFailures := 0
	previousStatus := model.HealthStatusUnknown
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			result := checker.Check(ctx)

			// Store result
			if r.store != nil {
				if err := r.store.AppendHealthResult(ctx, r.serviceID, result); err != nil {
					slog.Debug("Failed to store health result", "error", err)
				}
			}

			if result.Status == model.HealthStatusUnhealthy {
				consecutiveFailures++
				r.healthMu.Lock()
				r.lastHealthy = false
				r.healthMu.Unlock()
				retries := cfg.Retries
				if retries == 0 {
					retries = 3
				}
				if consecutiveFailures >= retries {
					slog.Warn("Health check failed", "service", r.serviceID, "type", checker.Type(), "failures", consecutiveFailures, "message", result.Message)
					select {
					case r.failureCh <- result:
					default:
					}
				}
			} else if result.Status == model.HealthStatusHealthy {
				if previousStatus == model.HealthStatusUnhealthy {
					slog.Info("Health check recovered", "service", r.serviceID, "type", checker.Type())
				}
				consecutiveFailures = 0
				r.healthMu.Lock()
				r.lastHealthy = true
				r.healthMu.Unlock()
			}
			previousStatus = result.Status

		case <-ctx.Done():
			return
		}
	}
}
