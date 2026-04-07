package api

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/hilman2/ELNSSM/internal/config"
	"github.com/hilman2/ELNSSM/internal/process"
)

func (s *Server) handleGetConfig(w http.ResponseWriter, r *http.Request) {
	// Return config with sensitive fields redacted
	sanitized := map[string]any{
		"guardian": s.cfg.Guardian,
		"api": map[string]any{
			"listen":       s.cfg.API.Listen,
			"ip_whitelist": s.cfg.API.IPWhitelist,
			"auth": map[string]any{
				"enabled": s.cfg.API.Auth.Enabled,
				"type":    s.cfg.API.Auth.Type,
			},
		},
		"defaults":              s.cfg.Defaults,
		"notification_cooldown": s.cfg.NotifyCooldown,
		"smtp": map[string]any{
			"enabled":    s.cfg.SMTP.Enabled,
			"host":       s.cfg.SMTP.Host,
			"port":       s.cfg.SMTP.Port,
			"username":   s.cfg.SMTP.Username,
			"from":       s.cfg.SMTP.From,
			"tls":        s.cfg.SMTP.TLS,
			"recipients": s.cfg.SMTP.Recipients,
		},
		"webhooks": s.cfg.Webhooks,
		"telegram": map[string]any{
			"enabled":  s.cfg.Telegram.Enabled,
			"chat_ids": s.cfg.Telegram.ChatIDs,
			// bot_token redacted
		},
		"ntfy": map[string]any{
			"enabled":  s.cfg.Ntfy.Enabled,
			"server":   s.cfg.Ntfy.Server,
			"topic":    s.cfg.Ntfy.Topic,
			"priority": s.cfg.Ntfy.Priority,
			"tags":     s.cfg.Ntfy.Tags,
			"email":    s.cfg.Ntfy.Email,
			// token redacted
		},
		"cluster": map[string]any{
			"role":               s.cfg.Cluster.Role,
			"node_name":          s.cfg.Cluster.NodeName,
			"master_addr":        s.cfg.Cluster.MasterAddr,
			"heartbeat_interval": s.cfg.Cluster.HeartbeatInterval,
		},
	}

	writeJSON(w, http.StatusOK, sanitized)
}

// configUpdate is the request body for updating configuration.
type configUpdate struct {
	Guardian *struct {
		EnableNativeServices *bool `json:"enable_native_services"`
	} `json:"guardian,omitempty"`

	Cluster *struct {
		Role              string `json:"role"`
		MasterAddr        string `json:"master_addr"`
		MasterToken       string `json:"master_token"`
		NodeName          string `json:"node_name"`
		HeartbeatInterval string `json:"heartbeat_interval"`
	} `json:"cluster,omitempty"`

	SMTP *struct {
		Enabled    *bool    `json:"enabled"`
		Host       string   `json:"host"`
		Port       int      `json:"port"`
		Username   string   `json:"username"`
		Password   string   `json:"password"`
		From       string   `json:"from"`
		TLS        *bool    `json:"tls"`
		Recipients []string `json:"recipients"`
	} `json:"smtp,omitempty"`

	Webhooks []config.WebhookConfig `json:"webhooks,omitempty"`

	Telegram *struct {
		Enabled  *bool    `json:"enabled"`
		BotToken string   `json:"bot_token"`
		ChatIDs  []string `json:"chat_ids"`
	} `json:"telegram,omitempty"`

	Ntfy *struct {
		Enabled  *bool  `json:"enabled"`
		Server   string `json:"server"`
		Topic    string `json:"topic"`
		Token    string `json:"token"`
		Priority string `json:"priority"`
		Tags     string `json:"tags"`
		Email    string `json:"email"`
	} `json:"ntfy,omitempty"`

	NotifyCooldown string `json:"notification_cooldown,omitempty"`
}

func (s *Server) handleUpdateConfig(w http.ResponseWriter, r *http.Request) {
	var updates configUpdate
	if err := json.NewDecoder(r.Body).Decode(&updates); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_BODY", "Invalid request body")
		return
	}

	changed := false
	needsRestart := false

	if updates.Guardian != nil {
		if updates.Guardian.EnableNativeServices != nil {
			s.cfg.Guardian.EnableNativeServices = *updates.Guardian.EnableNativeServices
			needsRestart = true
		}
		changed = true
	}

	if updates.Cluster != nil {
		if updates.Cluster.Role != "" {
			s.cfg.Cluster.Role = updates.Cluster.Role
			needsRestart = true
		}
		if updates.Cluster.MasterAddr != "" {
			s.cfg.Cluster.MasterAddr = updates.Cluster.MasterAddr
		}
		if updates.Cluster.MasterToken != "" {
			encrypted, err := process.EncryptPassword(updates.Cluster.MasterToken)
			if err != nil {
				slog.Error("Failed to encrypt cluster master token", "error", err)
				writeError(w, http.StatusInternalServerError, "ENCRYPT_FAILED", "Failed to encrypt master token")
				return
			}
			s.cfg.Cluster.EncryptedMasterToken = encrypted
			s.cfg.Cluster.MasterToken = updates.Cluster.MasterToken
		}
		if updates.Cluster.NodeName != "" {
			s.cfg.Cluster.NodeName = updates.Cluster.NodeName
		}
		if updates.Cluster.HeartbeatInterval != "" {
			s.cfg.Cluster.HeartbeatInterval = updates.Cluster.HeartbeatInterval
		}
		changed = true
		needsRestart = true
	}

	if updates.SMTP != nil {
		if updates.SMTP.Enabled != nil {
			s.cfg.SMTP.Enabled = *updates.SMTP.Enabled
		}
		if updates.SMTP.Host != "" {
			s.cfg.SMTP.Host = updates.SMTP.Host
		}
		if updates.SMTP.Port > 0 {
			s.cfg.SMTP.Port = updates.SMTP.Port
		}
		if updates.SMTP.Username != "" {
			s.cfg.SMTP.Username = updates.SMTP.Username
		}
		if updates.SMTP.Password != "" {
			// Encrypt the password via DPAPI before storing
			encrypted, err := process.EncryptPassword(updates.SMTP.Password)
			if err != nil {
				slog.Error("Failed to encrypt SMTP password", "error", err)
				writeError(w, http.StatusInternalServerError, "ENCRYPT_FAILED", "Failed to encrypt password")
				return
			}
			s.cfg.SMTP.EncryptedPassword = encrypted
			s.cfg.SMTP.Password = updates.SMTP.Password // keep in memory for runtime
		}
		if updates.SMTP.From != "" {
			s.cfg.SMTP.From = updates.SMTP.From
		}
		if updates.SMTP.TLS != nil {
			s.cfg.SMTP.TLS = *updates.SMTP.TLS
		}
		if updates.SMTP.Recipients != nil {
			s.cfg.SMTP.Recipients = updates.SMTP.Recipients
		}
		changed = true
	}

	if updates.Webhooks != nil {
		s.cfg.Webhooks = updates.Webhooks
		changed = true
	}

	if updates.Telegram != nil {
		if updates.Telegram.Enabled != nil {
			s.cfg.Telegram.Enabled = *updates.Telegram.Enabled
		}
		if updates.Telegram.BotToken != "" {
			// Encrypt the bot token via DPAPI before storing
			encrypted, err := process.EncryptPassword(updates.Telegram.BotToken)
			if err != nil {
				slog.Error("Failed to encrypt Telegram bot token", "error", err)
				writeError(w, http.StatusInternalServerError, "ENCRYPT_FAILED", "Failed to encrypt bot token")
				return
			}
			s.cfg.Telegram.EncryptedBotToken = encrypted
			s.cfg.Telegram.BotToken = updates.Telegram.BotToken // keep in memory for runtime
		}
		if updates.Telegram.ChatIDs != nil {
			s.cfg.Telegram.ChatIDs = updates.Telegram.ChatIDs
		}
		changed = true
	}

	if updates.Ntfy != nil {
		if updates.Ntfy.Enabled != nil {
			s.cfg.Ntfy.Enabled = *updates.Ntfy.Enabled
		}
		if updates.Ntfy.Server != "" {
			s.cfg.Ntfy.Server = updates.Ntfy.Server
		}
		if updates.Ntfy.Topic != "" {
			s.cfg.Ntfy.Topic = updates.Ntfy.Topic
		}
		if updates.Ntfy.Token != "" {
			encrypted, err := process.EncryptPassword(updates.Ntfy.Token)
			if err != nil {
				slog.Error("Failed to encrypt ntfy token", "error", err)
				writeError(w, http.StatusInternalServerError, "ENCRYPT_FAILED", "Failed to encrypt ntfy token")
				return
			}
			s.cfg.Ntfy.EncryptedToken = encrypted
			s.cfg.Ntfy.Token = updates.Ntfy.Token
		}
		if updates.Ntfy.Priority != "" {
			s.cfg.Ntfy.Priority = updates.Ntfy.Priority
		}
		s.cfg.Ntfy.Tags = updates.Ntfy.Tags
		s.cfg.Ntfy.Email = updates.Ntfy.Email
		changed = true
	}

	if updates.NotifyCooldown != "" {
		s.cfg.NotifyCooldown = updates.NotifyCooldown
		changed = true
	}

	if changed {
		// Clear plaintext secrets before saving to disk
		savedSMTPPass := s.cfg.SMTP.Password
		savedTGToken := s.cfg.Telegram.BotToken
		savedNtfyToken := s.cfg.Ntfy.Token
		savedMasterToken := s.cfg.Cluster.MasterToken
		s.cfg.SMTP.Password = ""
		s.cfg.Telegram.BotToken = ""
		s.cfg.Ntfy.Token = ""
		s.cfg.Cluster.MasterToken = ""

		cfgPath := s.cfg.FilePath
		if cfgPath == "" {
			cfgPath = config.DefaultConfigPath()
		}
		if err := s.cfg.Save(cfgPath); err != nil {
			slog.Error("Failed to save config", "error", err)
		}

		// Restore plaintext for runtime
		s.cfg.SMTP.Password = savedSMTPPass
		s.cfg.Telegram.BotToken = savedTGToken
		s.cfg.Ntfy.Token = savedNtfyToken
		s.cfg.Cluster.MasterToken = savedMasterToken
	}

	msg := "Configuration updated."
	if needsRestart {
		msg += " Some changes require a Guardian restart to take effect."
	}
	writeJSON(w, http.StatusOK, map[string]string{
		"message": msg,
	})
}
