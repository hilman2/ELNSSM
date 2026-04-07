// Package health implements service health checks (HTTP, TCP, script)
// and a runner that schedules them and tracks history per service.
package health

import (
	"context"
	"fmt"

	"github.com/hilman2/ELNSSM/internal/model"
)

// Checker performs a single health check and returns the result.
type Checker interface {
	Check(ctx context.Context) model.HealthCheckResult
	Type() model.HealthCheckType
}

// NewChecker creates the appropriate Checker from a config.
func NewChecker(cfg model.HealthCheckConfig) (Checker, error) {
	switch cfg.Type {
	case model.HealthCheckHTTP:
		return NewHTTPChecker(cfg), nil
	case model.HealthCheckTCP:
		return NewTCPChecker(cfg), nil
	case model.HealthCheckScript:
		return NewScriptChecker(cfg), nil
	default:
		return nil, fmt.Errorf("unknown health check type: %s", cfg.Type)
	}
}
