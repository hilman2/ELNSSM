package notify

import (
	"crypto/tls"
	"fmt"
	"net/smtp"
	"strings"

	"github.com/hilman2/ELNSSM/internal/config"
	"github.com/hilman2/ELNSSM/internal/model"
)

// EmailNotifier sends notifications via SMTP.
type EmailNotifier struct {
	cfg config.SMTPConfig
}

// NewEmailNotifier creates a new email notifier.
func NewEmailNotifier(cfg config.SMTPConfig) *EmailNotifier {
	return &EmailNotifier{cfg: cfg}
}

func (n *EmailNotifier) Name() string {
	return "email"
}

func (n *EmailNotifier) Send(event model.Event) error {
	if len(n.cfg.Recipients) == 0 {
		return nil
	}

	subject := fmt.Sprintf("[ELNSSM] %s: %s", event.Type, event.ServiceID)
	body := fmt.Sprintf("Event: %s\nService: %s\nTime: %s\n\n%s",
		event.Type, event.ServiceID, event.Timestamp.Format("2006-01-02 15:04:05"), event.Message)

	msg := fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: %s\r\nContent-Type: text/plain; charset=UTF-8\r\n\r\n%s",
		n.cfg.From,
		strings.Join(n.cfg.Recipients, ", "),
		subject,
		body,
	)

	addr := fmt.Sprintf("%s:%d", n.cfg.Host, n.cfg.Port)

	var auth smtp.Auth
	if n.cfg.Username != "" {
		auth = smtp.PlainAuth("", n.cfg.Username, n.cfg.Password, n.cfg.Host)
	}

	if n.cfg.TLS {
		return sendMailTLS(addr, auth, n.cfg.From, n.cfg.Recipients, []byte(msg), n.cfg.Host)
	}
	return smtp.SendMail(addr, auth, n.cfg.From, n.cfg.Recipients, []byte(msg))
}

func sendMailTLS(addr string, auth smtp.Auth, from string, to []string, msg []byte, host string) error {
	tlsConfig := &tls.Config{ServerName: host}
	conn, err := tls.Dial("tcp", addr, tlsConfig)
	if err != nil {
		return fmt.Errorf("TLS dial: %w", err)
	}

	client, err := smtp.NewClient(conn, host)
	if err != nil {
		return fmt.Errorf("SMTP client: %w", err)
	}
	defer client.Close()

	if auth != nil {
		if err := client.Auth(auth); err != nil {
			return fmt.Errorf("SMTP auth: %w", err)
		}
	}

	if err := client.Mail(from); err != nil {
		return fmt.Errorf("SMTP MAIL: %w", err)
	}
	for _, addr := range to {
		if err := client.Rcpt(addr); err != nil {
			return fmt.Errorf("SMTP RCPT: %w", err)
		}
	}

	w, err := client.Data()
	if err != nil {
		return fmt.Errorf("SMTP DATA: %w", err)
	}
	if _, err := w.Write(msg); err != nil {
		return fmt.Errorf("writing message: %w", err)
	}
	if err := w.Close(); err != nil {
		return fmt.Errorf("closing data writer: %w", err)
	}

	return client.Quit()
}
