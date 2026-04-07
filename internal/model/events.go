package model

import "time"

// EventType identifies the kind of system event.
type EventType string

const (
	EventServiceStarted            EventType = "service.started"
	EventServiceStopped            EventType = "service.stopped"
	EventServiceCrashed            EventType = "service.crashed"
	EventServiceRestarted          EventType = "service.restarted"
	EventServiceAdded              EventType = "service.added"
	EventServiceRemoved            EventType = "service.removed"
	EventServiceConfigChanged      EventType = "service.config_changed"
	EventHealthCheckFailed         EventType = "health_check.failed"
	EventHealthCheckRecovered      EventType = "health_check.recovered"
	EventRestartLimitReached       EventType = "restart.limit_reached"
	EventResourceThresholdBreached EventType = "resource.threshold_breached"
	EventHookExecuted              EventType = "hook.executed"
	EventHookFailed                EventType = "hook.failed"
	EventScheduleTriggered         EventType = "schedule.triggered"
)

// Event represents a system event related to a managed service.
type Event struct {
	ID        string         `json:"id"`
	Type      EventType      `json:"type"`
	ServiceID string         `json:"service_id"`
	Timestamp time.Time      `json:"timestamp"`
	Message   string         `json:"message"`
	Data      map[string]any `json:"data,omitempty"`
}
