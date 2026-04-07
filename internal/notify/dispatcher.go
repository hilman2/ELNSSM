package notify

import (
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/hilman2/ELNSSM/internal/config"
	"github.com/hilman2/ELNSSM/internal/model"
)

// Dispatcher routes events to all configured notifiers with cooldown logic.
type Dispatcher struct {
	notifiers        []Notifier
	cooldowns        map[string]time.Time
	cooldownDuration time.Duration
	mu               sync.RWMutex
}

// NewDispatcher creates a new notification dispatcher from config.
func NewDispatcher(cfg *config.Config) *Dispatcher {
	cooldown := 5 * time.Minute
	if cfg.NotifyCooldown != "" {
		if d, err := time.ParseDuration(cfg.NotifyCooldown); err == nil {
			cooldown = d
		}
	}

	d := &Dispatcher{
		cooldowns:        make(map[string]time.Time),
		cooldownDuration: cooldown,
	}

	// Add email notifier
	if cfg.SMTP.Enabled {
		d.notifiers = append(d.notifiers, NewEmailNotifier(cfg.SMTP))
	}

	// Add webhook notifiers
	for _, wh := range cfg.Webhooks {
		if wh.Enabled {
			d.notifiers = append(d.notifiers, NewWebhookNotifier(wh))
		}
	}

	// Add Telegram notifier
	if cfg.Telegram.Enabled && cfg.Telegram.BotToken != "" {
		d.notifiers = append(d.notifiers, NewTelegramNotifier(cfg.Telegram))
	}

	// Add ntfy notifier
	if cfg.Ntfy.Enabled && cfg.Ntfy.Topic != "" {
		d.notifiers = append(d.notifiers, NewNtfyNotifier(cfg.Ntfy))
	}

	return d
}

// Dispatch sends an event to all notifiers, respecting cooldown.
func (d *Dispatcher) Dispatch(event model.Event) {
	d.mu.Lock()
	defer d.mu.Unlock()

	key := fmt.Sprintf("%s:%s", event.ServiceID, event.Type)
	if lastSent, ok := d.cooldowns[key]; ok {
		if time.Since(lastSent) < d.cooldownDuration {
			slog.Debug("Notification suppressed (cooldown)", "key", key)
			return
		}
	}

	for _, n := range d.notifiers {
		if err := n.Send(event); err != nil {
			slog.Error("Failed to send notification", "notifier", n.Name(), "error", err)
		} else {
			slog.Info("Notification sent", "notifier", n.Name(), "event", event.Type, "service", event.ServiceID)
		}
	}

	d.cooldowns[key] = time.Now()
}
