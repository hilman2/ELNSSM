package logging

import (
	"fmt"
	"log/slog"

	"golang.org/x/sys/windows/svc/eventlog"
)

// EventLogger writes important events to the Windows Event Log.
type EventLogger struct {
	log *eventlog.Log
}

// NewEventLogger opens a connection to the Windows Event Log.
func NewEventLogger(source string) (*EventLogger, error) {
	log, err := eventlog.Open(source)
	if err != nil {
		return nil, fmt.Errorf("opening event log: %w", err)
	}
	return &EventLogger{log: log}, nil
}

// Info writes an informational event.
func (e *EventLogger) Info(eventID uint32, msg string) {
	if err := e.log.Info(eventID, msg); err != nil {
		slog.Debug("Failed to write event log info", "error", err)
	}
}

// Warning writes a warning event.
func (e *EventLogger) Warning(eventID uint32, msg string) {
	if err := e.log.Warning(eventID, msg); err != nil {
		slog.Debug("Failed to write event log warning", "error", err)
	}
}

// Error writes an error event.
func (e *EventLogger) Error(eventID uint32, msg string) {
	if err := e.log.Error(eventID, msg); err != nil {
		slog.Debug("Failed to write event log error", "error", err)
	}
}

// Close releases the event log connection.
func (e *EventLogger) Close() error {
	return e.log.Close()
}
