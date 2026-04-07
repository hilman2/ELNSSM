package health

import (
	"sync"

	"github.com/hilman2/ELNSSM/internal/model"
)

// RingBuffer stores the last N health check results in memory.
type RingBuffer struct {
	results []model.HealthCheckResult
	size    int
	pos     int
	full    bool
	mu      sync.RWMutex
}

// NewRingBuffer creates a new ring buffer with the given capacity.
func NewRingBuffer(size int) *RingBuffer {
	return &RingBuffer{
		results: make([]model.HealthCheckResult, size),
		size:    size,
	}
}

// Add adds a result to the ring buffer.
func (rb *RingBuffer) Add(result model.HealthCheckResult) {
	rb.mu.Lock()
	defer rb.mu.Unlock()

	rb.results[rb.pos] = result
	rb.pos = (rb.pos + 1) % rb.size
	if rb.pos == 0 {
		rb.full = true
	}
}

// GetAll returns all results in chronological order.
func (rb *RingBuffer) GetAll() []model.HealthCheckResult {
	rb.mu.RLock()
	defer rb.mu.RUnlock()

	if !rb.full {
		result := make([]model.HealthCheckResult, rb.pos)
		copy(result, rb.results[:rb.pos])
		return result
	}

	result := make([]model.HealthCheckResult, rb.size)
	copy(result, rb.results[rb.pos:])
	copy(result[rb.size-rb.pos:], rb.results[:rb.pos])
	return result
}

// Latest returns the most recent result, if any.
func (rb *RingBuffer) Latest() (model.HealthCheckResult, bool) {
	rb.mu.RLock()
	defer rb.mu.RUnlock()

	if rb.pos == 0 && !rb.full {
		return model.HealthCheckResult{}, false
	}

	idx := rb.pos - 1
	if idx < 0 {
		idx = rb.size - 1
	}
	return rb.results[idx], true
}
