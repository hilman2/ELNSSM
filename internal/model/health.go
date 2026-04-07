package model

import "time"

// HealthStatus represents the overall health of a service.
type HealthStatus string

const (
	HealthStatusHealthy   HealthStatus = "healthy"
	HealthStatusUnhealthy HealthStatus = "unhealthy"
	HealthStatusUnknown   HealthStatus = "unknown"
)

// HealthCheckResult stores the outcome of a single health check execution.
type HealthCheckResult struct {
	CheckType  HealthCheckType `json:"check_type"`
	Status     HealthStatus    `json:"status"`
	Timestamp  time.Time       `json:"timestamp"`
	Duration   time.Duration   `json:"duration"`
	Message    string          `json:"message,omitempty"`
	StatusCode int             `json:"status_code,omitempty"`
}
