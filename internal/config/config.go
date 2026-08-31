// Package config loads and writes the YAML configuration files used by
// ELNSSM (the global Guardian config plus per-service definitions) and
// supports hot-reloading them via fsnotify.
package config

import (
	"fmt"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/hilman2/ELNSSM/internal/process"
)

// Config is the global ELNSSM configuration.
type Config struct {
	Guardian       GuardianConfig  `yaml:"guardian" json:"guardian"`
	API            APIConfig       `yaml:"api" json:"api"`
	Cluster        ClusterConfig   `yaml:"cluster" json:"cluster"`
	Defaults       DefaultsConfig  `yaml:"defaults" json:"defaults"`
	SMTP           SMTPConfig      `yaml:"smtp" json:"smtp"`
	Webhooks       []WebhookConfig `yaml:"webhooks" json:"webhooks"`
	Telegram       TelegramConfig  `yaml:"telegram" json:"telegram"`
	Ntfy           NtfyConfig      `yaml:"ntfy" json:"ntfy"`
	NotifyCooldown string          `yaml:"notification_cooldown" json:"notification_cooldown"`

	// FilePath is the path this config was loaded from (not serialized).
	FilePath string `yaml:"-" json:"-"`
}

// ClusterConfig holds Master/Slave cluster settings.
type ClusterConfig struct {
	Role                 string `yaml:"role" json:"role"`
	MasterAddr           string `yaml:"master_addr,omitempty" json:"master_addr,omitempty"`
	MasterToken          string `yaml:"master_token,omitempty" json:"master_token,omitempty"`
	EncryptedMasterToken string `yaml:"encrypted_master_token,omitempty" json:"-"`
	SlaveToken           string `yaml:"slave_token,omitempty" json:"-"`
	EncryptedSlaveToken  string `yaml:"encrypted_slave_token,omitempty" json:"-"`
	NodeName             string `yaml:"node_name,omitempty" json:"node_name,omitempty"`
	HeartbeatInterval    string `yaml:"heartbeat_interval,omitempty" json:"heartbeat_interval,omitempty"`
}

// GuardianConfig holds settings for the Guardian service itself.
type GuardianConfig struct {
	DataDir              string `yaml:"data_dir" json:"data_dir"`
	LogLevel             string `yaml:"log_level" json:"log_level"`
	EnableNativeServices bool   `yaml:"enable_native_services" json:"enable_native_services"`
}

// APIConfig holds HTTP API server settings.
type APIConfig struct {
	Enabled     bool       `yaml:"enabled" json:"enabled"`
	Listen      string     `yaml:"listen" json:"listen"`
	IPWhitelist []string   `yaml:"ip_whitelist" json:"ip_whitelist"`
	Auth        AuthConfig `yaml:"auth" json:"auth"`
}

// AuthConfig holds authentication settings.
type AuthConfig struct {
	Enabled      bool   `yaml:"enabled" json:"enabled"`
	Type         string `yaml:"type" json:"type"`
	Username     string `yaml:"username,omitempty" json:"username,omitempty"`
	PasswordHash string `yaml:"password_hash,omitempty" json:"-"`
	TokenHash    string `yaml:"token_hash,omitempty" json:"-"`

	// AllowLocalBypass lets callers from 127.0.0.1 and ::1 through without
	// credentials, as the identity "local-admin". It defaults to true: on a
	// typical Windows Server only administrators can sign in interactively,
	// so anyone able to open a loopback socket already holds the privileges
	// the API would hand them, and requiring a token there only costs the
	// operator convenience.
	//
	// That assumption breaks in two deployments, and both should set this to
	// false. On an RDS or terminal server, ordinary users are signed in and
	// reach loopback like anyone else. And a service account such as
	// NetworkService, or a compromised IIS worker, reaches loopback with no
	// interactive logon at all. Because the Guardian starts processes as
	// LocalSystem, the bypass hands either of them a way up.
	AllowLocalBypass bool `yaml:"allow_local_bypass" json:"allow_local_bypass"`
}

// DefaultsConfig holds default values for service settings.
type DefaultsConfig struct {
	StopTimeout   string              `yaml:"stop_timeout" json:"stop_timeout"`
	StopSignal    string              `yaml:"stop_signal" json:"stop_signal"`
	Priority      string              `yaml:"priority" json:"priority"`
	RestartPolicy RestartPolicyYAML   `yaml:"restart_policy" json:"restart_policy"`
	Logging       LoggingDefaultsYAML `yaml:"logging" json:"logging"`
}

// RestartPolicyYAML is the YAML-friendly representation of restart policy defaults.
type RestartPolicyYAML struct {
	Mode                string  `yaml:"mode" json:"mode"`
	Delay               string  `yaml:"delay" json:"delay"`
	MaxRetries          int     `yaml:"max_retries" json:"max_retries"`
	RetryWindow         string  `yaml:"retry_window" json:"retry_window"`
	BackoffMultiplier   float64 `yaml:"backoff_multiplier" json:"backoff_multiplier"`
	MaxBackoff          string  `yaml:"max_backoff" json:"max_backoff"`
	RestartOnHealthFail bool    `yaml:"restart_on_health_fail" json:"restart_on_health_fail"`
	ScheduledRestart    string  `yaml:"scheduled_restart,omitempty" json:"scheduled_restart,omitempty"`
}

// LoggingDefaultsYAML is the YAML-friendly representation of logging defaults.
type LoggingDefaultsYAML struct {
	MaxSize       int64  `yaml:"max_size" json:"max_size"`
	MaxAge        string `yaml:"max_age" json:"max_age"`
	MaxBackups    int    `yaml:"max_backups" json:"max_backups"`
	Compress      bool   `yaml:"compress" json:"compress"`
	CombineOutput bool   `yaml:"combine_output" json:"combine_output"`
}

// SMTPConfig holds email notification settings.
type SMTPConfig struct {
	Enabled           bool     `yaml:"enabled" json:"enabled"`
	Host              string   `yaml:"host" json:"host"`
	Port              int      `yaml:"port" json:"port"`
	Username          string   `yaml:"username" json:"username"`
	Password          string   `yaml:"password,omitempty" json:"-"`
	EncryptedPassword string   `yaml:"encrypted_password,omitempty" json:"-"`
	From              string   `yaml:"from" json:"from"`
	TLS               bool     `yaml:"tls" json:"tls"`
	Recipients        []string `yaml:"recipients" json:"recipients"`
}

// WebhookConfig holds a single webhook notification target.
type WebhookConfig struct {
	Name         string            `yaml:"name" json:"name"`
	Enabled      bool              `yaml:"enabled" json:"enabled"`
	URL          string            `yaml:"url" json:"url"`
	Method       string            `yaml:"method" json:"method"`
	Headers      map[string]string `yaml:"headers" json:"headers"`
	BodyTemplate string            `yaml:"body_template" json:"body_template"`
	Events       []string          `yaml:"events" json:"events"`
}

// TelegramConfig holds Telegram Bot notification settings.
type TelegramConfig struct {
	Enabled           bool     `yaml:"enabled" json:"enabled"`
	BotToken          string   `yaml:"bot_token,omitempty" json:"-"`
	EncryptedBotToken string   `yaml:"encrypted_bot_token,omitempty" json:"-"`
	ChatIDs           []string `yaml:"chat_ids" json:"chat_ids"`
}

// NtfyConfig holds ntfy.sh notification settings.
type NtfyConfig struct {
	Enabled        bool   `yaml:"enabled" json:"enabled"`
	Server         string `yaml:"server" json:"server"`
	Topic          string `yaml:"topic" json:"topic"`
	Token          string `yaml:"token,omitempty" json:"-"`
	EncryptedToken string `yaml:"encrypted_token,omitempty" json:"-"`
	Priority       string `yaml:"priority,omitempty" json:"priority,omitempty"`
	Tags           string `yaml:"tags,omitempty" json:"tags,omitempty"`
	Email          string `yaml:"email,omitempty" json:"email,omitempty"`
}

// Load reads and parses the global configuration from a YAML file.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading config file: %w", err)
	}

	cfg := DefaultConfig()
	cfg.FilePath = path
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parsing config file: %w", err)
	}

	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("validating config: %w", err)
	}

	// Migrate plaintext secrets to encrypted
	if err := cfg.EncryptSecrets(); err != nil {
		slog.Warn("Could not encrypt secrets in config", "error", err)
	} else {
		// Re-save config with encrypted secrets
		if saveErr := cfg.Save(path); saveErr != nil {
			slog.Warn("Could not re-save config after encrypting secrets", "error", saveErr)
		}
	}

	// Decrypt secrets for runtime use
	if err := cfg.DecryptSecrets(); err != nil {
		slog.Warn("Could not decrypt secrets", "error", err)
	}

	return cfg, nil
}

// Save writes the configuration to a YAML file.
func (c *Config) Save(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return fmt.Errorf("creating config directory: %w", err)
	}

	data, err := yaml.Marshal(c)
	if err != nil {
		return fmt.Errorf("marshaling config: %w", err)
	}

	if err := os.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("writing config file: %w", err)
	}

	return nil
}

// Validate checks the configuration for errors.
func (c *Config) Validate() error {
	// Validate IP whitelist: no CIDR, only individual IPs
	for _, ip := range c.API.IPWhitelist {
		if strings.Contains(ip, "/") {
			return fmt.Errorf("CIDR notation not allowed in ip_whitelist: %q (use individual IPs)", ip)
		}
		if net.ParseIP(ip) == nil {
			return fmt.Errorf("invalid IP address in ip_whitelist: %q", ip)
		}
	}

	// Validate auth config
	if c.API.Auth.Enabled {
		switch c.API.Auth.Type {
		case "token":
			if c.API.Auth.TokenHash == "" {
				return fmt.Errorf("auth is enabled with type 'token' but token_hash is empty (run: elnssm reset-token)")
			}
		case "basic":
			if c.API.Auth.Username == "" || c.API.Auth.PasswordHash == "" {
				return fmt.Errorf("auth is enabled with type 'basic' but username or password_hash is empty")
			}
		case "sspi":
			// No extra fields required. token_hash is optional for Bearer fallback.
		default:
			return fmt.Errorf("unknown auth type: %q (supported: token, basic, sspi)", c.API.Auth.Type)
		}
	}

	return nil
}

// EncryptSecrets encrypts plaintext secrets using DPAPI and clears the plaintext fields.
func (c *Config) EncryptSecrets() error {
	if c.SMTP.Password != "" {
		encrypted, err := process.EncryptPassword(c.SMTP.Password)
		if err != nil {
			return fmt.Errorf("encrypting SMTP password: %w", err)
		}
		c.SMTP.EncryptedPassword = encrypted
		c.SMTP.Password = ""
	}

	if c.Telegram.BotToken != "" {
		encrypted, err := process.EncryptPassword(c.Telegram.BotToken)
		if err != nil {
			return fmt.Errorf("encrypting Telegram bot token: %w", err)
		}
		c.Telegram.EncryptedBotToken = encrypted
		c.Telegram.BotToken = ""
	}

	if c.Ntfy.Token != "" {
		encrypted, err := process.EncryptPassword(c.Ntfy.Token)
		if err != nil {
			return fmt.Errorf("encrypting ntfy token: %w", err)
		}
		c.Ntfy.EncryptedToken = encrypted
		c.Ntfy.Token = ""
	}

	if c.Cluster.MasterToken != "" {
		encrypted, err := process.EncryptPassword(c.Cluster.MasterToken)
		if err != nil {
			return fmt.Errorf("encrypting cluster master token: %w", err)
		}
		c.Cluster.EncryptedMasterToken = encrypted
		c.Cluster.MasterToken = ""
	}

	if c.Cluster.SlaveToken != "" {
		encrypted, err := process.EncryptPassword(c.Cluster.SlaveToken)
		if err != nil {
			return fmt.Errorf("encrypting cluster slave token: %w", err)
		}
		c.Cluster.EncryptedSlaveToken = encrypted
		c.Cluster.SlaveToken = ""
	}

	return nil
}

// DecryptSecrets decrypts DPAPI-encrypted secrets into the plaintext fields for runtime use.
func (c *Config) DecryptSecrets() error {
	if c.SMTP.EncryptedPassword != "" && c.SMTP.Password == "" {
		plaintext, err := process.DecryptPassword(c.SMTP.EncryptedPassword)
		if err != nil {
			return fmt.Errorf("decrypting SMTP password: %w", err)
		}
		c.SMTP.Password = plaintext
	}

	if c.Telegram.EncryptedBotToken != "" && c.Telegram.BotToken == "" {
		plaintext, err := process.DecryptPassword(c.Telegram.EncryptedBotToken)
		if err != nil {
			return fmt.Errorf("decrypting Telegram bot token: %w", err)
		}
		c.Telegram.BotToken = plaintext
	}

	if c.Ntfy.EncryptedToken != "" && c.Ntfy.Token == "" {
		plaintext, err := process.DecryptPassword(c.Ntfy.EncryptedToken)
		if err != nil {
			return fmt.Errorf("decrypting ntfy token: %w", err)
		}
		c.Ntfy.Token = plaintext
	}

	if c.Cluster.EncryptedMasterToken != "" && c.Cluster.MasterToken == "" {
		plaintext, err := process.DecryptPassword(c.Cluster.EncryptedMasterToken)
		if err != nil {
			return fmt.Errorf("decrypting cluster master token: %w", err)
		}
		c.Cluster.MasterToken = plaintext
	}

	if c.Cluster.EncryptedSlaveToken != "" && c.Cluster.SlaveToken == "" {
		plaintext, err := process.DecryptPassword(c.Cluster.EncryptedSlaveToken)
		if err != nil {
			return fmt.Errorf("decrypting cluster slave token: %w", err)
		}
		c.Cluster.SlaveToken = plaintext
	}

	return nil
}

// ConfigDir returns the path to the config directory.
func (c *Config) ConfigDir() string {
	return filepath.Join(c.Guardian.DataDir, "config")
}

// ServicesDir returns the path to the per-service config directory.
func (c *Config) ServicesDir() string {
	return filepath.Join(c.Guardian.DataDir, "config", "services")
}

// DataPath returns the path to the bbolt database.
func (c *Config) DataPath() string {
	return filepath.Join(c.Guardian.DataDir, "data", "elnssm.db")
}

// LogsDir returns the path to the logs directory.
func (c *Config) LogsDir() string {
	return filepath.Join(c.Guardian.DataDir, "logs")
}

// ServiceLogDir returns the log directory for a specific service.
func (c *Config) ServiceLogDir(serviceID string) string {
	return filepath.Join(c.Guardian.DataDir, "logs", serviceID)
}
