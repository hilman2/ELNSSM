package manager

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/hilman2/ELNSSM/internal/model"
)

// TopologicalSort returns service IDs in dependency order (dependencies first).
// Returns an error if a cycle is detected.
func TopologicalSort(services map[string]*ManagedService) ([]string, error) {
	// Build adjacency: service -> services it depends on
	graph := make(map[string][]string)
	allIDs := make(map[string]bool)

	for id, ms := range services {
		allIDs[id] = true
		for _, dep := range ms.Config.Dependencies {
			graph[id] = append(graph[id], dep.ServiceID)
		}
	}

	// Kahn's algorithm
	inDegree := make(map[string]int)
	for id := range allIDs {
		inDegree[id] = 0
	}

	// Reverse graph: for each dependency edge A->B (A depends on B), B blocks A
	reverse := make(map[string][]string)
	for id, deps := range graph {
		for _, dep := range deps {
			reverse[dep] = append(reverse[dep], id)
			inDegree[id]++
		}
	}

	// Find all nodes with no dependencies
	var queue []string
	for id := range allIDs {
		if inDegree[id] == 0 {
			queue = append(queue, id)
		}
	}

	var sorted []string
	for len(queue) > 0 {
		node := queue[0]
		queue = queue[1:]
		sorted = append(sorted, node)

		for _, dependent := range reverse[node] {
			inDegree[dependent]--
			if inDegree[dependent] == 0 {
				queue = append(queue, dependent)
			}
		}
	}

	if len(sorted) != len(allIDs) {
		return nil, fmt.Errorf("circular dependency detected among services")
	}

	return sorted, nil
}

// DetectCycles checks for circular dependencies.
func DetectCycles(services map[string]*ManagedService) error {
	_, err := TopologicalSort(services)
	return err
}

// WaitForDependencies blocks until all dependencies of the given service are met.
// Returns an error if a dependency times out or the context is cancelled.
func (m *Manager) WaitForDependencies(ctx context.Context, svc *model.Service) error {
	if len(svc.Dependencies) == 0 {
		return nil
	}

	for _, dep := range svc.Dependencies {
		timeout := dep.Timeout
		if timeout <= 0 {
			timeout = 5 * time.Minute
		}

		depCtx, cancel := context.WithTimeout(ctx, timeout)
		err := m.waitForSingleDependency(depCtx, svc.ID, dep)
		cancel()

		if err != nil {
			return fmt.Errorf("dependency %q (%s): %w", dep.ServiceID, dep.Type, err)
		}
	}

	return nil
}

func (m *Manager) waitForSingleDependency(ctx context.Context, serviceID string, dep model.ServiceDependency) error {
	slog.Info("Waiting for dependency", "service", serviceID, "depends_on", dep.ServiceID, "type", dep.Type)

	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("timeout waiting for %s to be %s", dep.ServiceID, dep.Type)
		case <-ticker.C:
			if m.isDependencyMet(dep) {
				slog.Info("Dependency met", "service", serviceID, "depends_on", dep.ServiceID, "type", dep.Type)
				return nil
			}
		}
	}
}

func (m *Manager) isDependencyMet(dep model.ServiceDependency) bool {
	m.mu.RLock()
	ms, ok := m.services[dep.ServiceID]
	m.mu.RUnlock()

	if !ok {
		return false
	}

	switch dep.Type {
	case model.DependencyRunning:
		return ms.Config.State == model.ServiceStateRunning

	case model.DependencyHealthy:
		if ms.Config.State != model.ServiceStateRunning {
			return false
		}
		// Check if the service has a health runner and last known status is healthy
		if ms.HealthRunner != nil {
			return ms.HealthRunner.IsHealthy()
		}
		// If no health checks configured, treat "running" as "healthy"
		return true

	default:
		return ms.Config.State == model.ServiceStateRunning
	}
}
