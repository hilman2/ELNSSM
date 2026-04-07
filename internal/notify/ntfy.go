package notify

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/hilman2/ELNSSM/internal/config"
	"github.com/hilman2/ELNSSM/internal/model"
)

// NtfyNotifier sends notifications via ntfy.sh (or a self-hosted ntfy server).
type NtfyNotifier struct {
	cfg    config.NtfyConfig
	client *http.Client
}

// NewNtfyNotifier creates a new ntfy notifier.
func NewNtfyNotifier(cfg config.NtfyConfig) *NtfyNotifier {
	return &NtfyNotifier{
		cfg: cfg,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

func (n *NtfyNotifier) Name() string {
	return "ntfy"
}

func (n *NtfyNotifier) Send(event model.Event) error {
	server := n.cfg.Server
	if server == "" {
		server = "https://ntfy.sh"
	}
	server = strings.TrimRight(server, "/")

	url := fmt.Sprintf("%s/%s", server, n.cfg.Topic)

	body := fmt.Sprintf("[%s] %s: %s",
		event.Type,
		event.ServiceID,
		event.Message,
	)

	req, err := http.NewRequest(http.MethodPost, url, strings.NewReader(body))
	if err != nil {
		return fmt.Errorf("ntfy: creating request: %w", err)
	}

	title := fmt.Sprintf("ELNSSM - %s", event.ServiceID)
	req.Header.Set("Title", title)

	if n.cfg.Priority != "" {
		req.Header.Set("Priority", n.cfg.Priority)
	}

	if n.cfg.Tags != "" {
		req.Header.Set("Tags", n.cfg.Tags)
	}

	if n.cfg.Email != "" {
		req.Header.Set("Email", n.cfg.Email)
	}

	if n.cfg.Token != "" {
		req.Header.Set("Authorization", "Bearer "+n.cfg.Token)
	}

	resp, err := n.client.Do(req)
	if err != nil {
		return fmt.Errorf("ntfy: sending notification: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("ntfy: server returned status %d", resp.StatusCode)
	}

	return nil
}
