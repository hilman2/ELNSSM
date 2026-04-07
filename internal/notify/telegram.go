package notify

import (
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/hilman2/ELNSSM/internal/config"
	"github.com/hilman2/ELNSSM/internal/model"
)

// TelegramNotifier sends notifications via Telegram Bot API.
type TelegramNotifier struct {
	cfg    config.TelegramConfig
	client *http.Client
}

// NewTelegramNotifier creates a new Telegram notifier.
func NewTelegramNotifier(cfg config.TelegramConfig) *TelegramNotifier {
	return &TelegramNotifier{
		cfg: cfg,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

func (n *TelegramNotifier) Name() string {
	return "telegram"
}

func (n *TelegramNotifier) Send(event model.Event) error {
	text := fmt.Sprintf("*[ELNSSM]* `%s`\n*Service:* `%s`\n*Message:* %s\n*Time:* %s",
		event.Type,
		event.ServiceID,
		escapeMarkdown(event.Message),
		event.Timestamp.Format("2006-01-02 15:04:05"),
	)

	apiURL := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", n.cfg.BotToken)

	for _, chatID := range n.cfg.ChatIDs {
		params := url.Values{}
		params.Set("chat_id", chatID)
		params.Set("text", text)
		params.Set("parse_mode", "Markdown")

		resp, err := n.client.PostForm(apiURL, params)
		if err != nil {
			return fmt.Errorf("telegram send to %s: %w", chatID, err)
		}
		resp.Body.Close()

		if resp.StatusCode >= 400 {
			return fmt.Errorf("telegram API returned status %d for chat %s", resp.StatusCode, chatID)
		}
	}

	return nil
}

// escapeMarkdown escapes special Markdown characters for Telegram.
func escapeMarkdown(s string) string {
	replacer := []string{
		"_", "\\_",
		"*", "\\*",
		"[", "\\[",
		"`", "\\`",
	}
	for i := 0; i < len(replacer); i += 2 {
		result := ""
		for _, c := range s {
			if string(c) == replacer[i] {
				result += replacer[i+1]
			} else {
				result += string(c)
			}
		}
		s = result
	}
	return s
}
