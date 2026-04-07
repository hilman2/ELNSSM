package notify

import "github.com/hilman2/ELNSSM/internal/model"

// Notifier sends a notification for a given event.
type Notifier interface {
	Name() string
	Send(event model.Event) error
}
