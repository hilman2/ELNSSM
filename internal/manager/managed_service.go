package manager

import (
	"sync"

	"github.com/hilman2/ELNSSM/internal/health"
	"github.com/hilman2/ELNSSM/internal/logging"
	"github.com/hilman2/ELNSSM/internal/model"
	"github.com/hilman2/ELNSSM/internal/process"
)

// ManagedService bundles a service config with its runtime components.
type ManagedService struct {
	Config          *model.Service
	Wrapper         *process.Wrapper
	HealthRunner    *health.Runner
	Capture         *logging.Capture
	ResourceMonitor *process.ResourceMonitor
	stopCh          chan struct{} // signals the monitor goroutine to stop
	mu              sync.Mutex
}

// NewManagedService creates a new ManagedService from a service config.
func NewManagedService(svc *model.Service) *ManagedService {
	return &ManagedService{
		Config: svc,
		stopCh: make(chan struct{}),
	}
}
