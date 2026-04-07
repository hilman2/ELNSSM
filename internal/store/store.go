// Package store defines the persistence interface used by the manager
// for runtime state and event history, and provides a bbolt-backed
// implementation along with simple schema migrations.
package store

import (
	"context"
	"time"

	"github.com/hilman2/ELNSSM/internal/model"
)

// ServiceRuntimeState holds volatile runtime data persisted across Guardian restarts.
type ServiceRuntimeState struct {
	State        model.ServiceState `json:"state"`
	PID          int                `json:"pid"`
	StartedAt    *time.Time         `json:"started_at,omitempty"`
	RestartCount int                `json:"restart_count"`
	LastExitCode int                `json:"last_exit_code"`
	LastError    string             `json:"last_error,omitempty"`
}

// EventFilter specifies criteria for querying events.
type EventFilter struct {
	ServiceID *string
	Type      *model.EventType
	Since     *time.Time
	Limit     int
}

// Store abstracts all persistence operations.
type Store interface {
	// Service config persistence
	ListServices(ctx context.Context) ([]*model.Service, error)
	GetService(ctx context.Context, id string) (*model.Service, error)
	SaveService(ctx context.Context, svc *model.Service) error
	DeleteService(ctx context.Context, id string) error

	// Runtime state persistence (survives Guardian restarts)
	GetServiceState(ctx context.Context, id string) (*ServiceRuntimeState, error)
	SaveServiceState(ctx context.Context, id string, state *ServiceRuntimeState) error

	// Events / audit log
	AppendEvent(ctx context.Context, event model.Event) error
	ListEvents(ctx context.Context, filter EventFilter) ([]model.Event, error)

	// Health check history
	AppendHealthResult(ctx context.Context, serviceID string, result model.HealthCheckResult) error
	GetHealthHistory(ctx context.Context, serviceID string, limit int) ([]model.HealthCheckResult, error)

	// Resource performance history
	AppendResourceSample(ctx context.Context, serviceID string, sample model.ResourceSample) error
	GetResourceHistory(ctx context.Context, serviceID string, from, to time.Time, maxPoints int) ([]model.ResourceSample, error)
	PruneResourceHistory(ctx context.Context, olderThan time.Time) error

	Close() error
}
