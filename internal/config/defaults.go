package config

import (
	"os"
	"path/filepath"
)

// DefaultDataDir returns the default data directory (%ProgramData%\ELNSSM).
func DefaultDataDir() string {
	programData := os.Getenv("ProgramData")
	if programData == "" {
		programData = `C:\ProgramData`
	}
	return filepath.Join(programData, "ELNSSM")
}

// DefaultConfig returns a Config populated with sensible defaults.
func DefaultConfig() *Config {
	return &Config{
		Guardian: GuardianConfig{
			DataDir:  DefaultDataDir(),
			LogLevel: "info",
		},
		API: APIConfig{
			Enabled: true,
			Listen:  "127.0.0.1:9100",
			IPWhitelist: []string{
				"127.0.0.1",
				"::1",
			},
			Auth: AuthConfig{
				Enabled:          true,
				Type:             "token",
				AllowLocalBypass: true,
			},
		},
		Defaults: DefaultsConfig{
			StopTimeout: "30s",
			StopSignal:  "ctrl_c",
			Priority:    "normal",
			RestartPolicy: RestartPolicyYAML{
				Mode:                "on_failure",
				Delay:               "5s",
				MaxRetries:          10,
				RetryWindow:         "1h",
				BackoffMultiplier:   2.0,
				MaxBackoff:          "5m",
				RestartOnHealthFail: true,
			},
			Logging: LoggingDefaultsYAML{
				MaxSize:       52428800, // 50 MB
				MaxAge:        "168h",   // 7 days
				MaxBackups:    5,
				Compress:      true,
				CombineOutput: false,
			},
		},
		SMTP: SMTPConfig{
			Enabled: false,
			Port:    587,
			TLS:     true,
		},
		NotifyCooldown: "5m",
	}
}

// DefaultConfigPath returns the default path to the global config file.
func DefaultConfigPath() string {
	return filepath.Join(DefaultDataDir(), "config", "elnssm.yaml")
}
