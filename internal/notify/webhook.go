package notify

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"text/template"
	"time"

	"github.com/hilman2/ELNSSM/internal/config"
	"github.com/hilman2/ELNSSM/internal/model"
)

// WebhookNotifier sends notifications to webhook endpoints.
type WebhookNotifier struct {
	cfg    config.WebhookConfig
	client *http.Client
}

// NewWebhookNotifier creates a new webhook notifier.
func NewWebhookNotifier(cfg config.WebhookConfig) *WebhookNotifier {
	return &WebhookNotifier{
		cfg: cfg,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

func (n *WebhookNotifier) Name() string {
	return fmt.Sprintf("webhook:%s", n.cfg.Name)
}

func (n *WebhookNotifier) Send(event model.Event) error {
	// Check if this webhook subscribes to this event type
	if len(n.cfg.Events) > 0 {
		found := false
		for _, e := range n.cfg.Events {
			if e == string(event.Type) {
				found = true
				break
			}
		}
		if !found {
			return nil
		}
	}

	// Render body template
	body, err := n.renderBody(event)
	if err != nil {
		return fmt.Errorf("rendering webhook body: %w", err)
	}

	method := n.cfg.Method
	if method == "" {
		method = "POST"
	}

	req, err := http.NewRequest(method, n.cfg.URL, strings.NewReader(body))
	if err != nil {
		return fmt.Errorf("creating webhook request: %w", err)
	}

	for k, v := range n.cfg.Headers {
		req.Header.Set(k, v)
	}

	resp, err := n.client.Do(req)
	if err != nil {
		return fmt.Errorf("sending webhook: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("webhook returned status %d", resp.StatusCode)
	}

	return nil
}

func (n *WebhookNotifier) renderBody(event model.Event) (string, error) {
	if n.cfg.BodyTemplate == "" {
		payload := map[string]string{
			"event":     string(event.Type),
			"service":   event.ServiceID,
			"message":   event.Message,
			"timestamp": event.Timestamp.Format(time.RFC3339),
		}
		buf, err := json.Marshal(payload)
		if err != nil {
			return "", fmt.Errorf("marshaling default webhook body: %w", err)
		}
		return string(buf), nil
	}

	tmpl, err := template.New("webhook").Parse(n.cfg.BodyTemplate)
	if err != nil {
		return "", fmt.Errorf("parsing template: %w", err)
	}

	data := map[string]string{
		"Event":       string(event.Type),
		"ServiceName": event.ServiceID,
		"Message":     event.Message,
		"Timestamp":   event.Timestamp.Format(time.RFC3339),
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("executing template: %w", err)
	}

	return buf.String(), nil
}
