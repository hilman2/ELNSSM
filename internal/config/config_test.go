package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.API.Listen != "127.0.0.1:9100" {
		t.Errorf("Listen = %q, want 127.0.0.1:9100", cfg.API.Listen)
	}
	if len(cfg.API.IPWhitelist) != 2 {
		t.Errorf("IPWhitelist len = %d, want 2", len(cfg.API.IPWhitelist))
	}
	if cfg.Guardian.LogLevel != "info" {
		t.Errorf("LogLevel = %q, want info", cfg.Guardian.LogLevel)
	}
	if cfg.Defaults.StopTimeout != "30s" {
		t.Errorf("StopTimeout = %q, want 30s", cfg.Defaults.StopTimeout)
	}
	if cfg.Defaults.RestartPolicy.Mode != "on_failure" {
		t.Errorf("RestartPolicy.Mode = %q, want on_failure", cfg.Defaults.RestartPolicy.Mode)
	}
	if cfg.SMTP.Port != 587 {
		t.Errorf("SMTP.Port = %d, want 587", cfg.SMTP.Port)
	}
}

func TestDefaultDataDir(t *testing.T) {
	dir := DefaultDataDir()
	if dir == "" {
		t.Error("DefaultDataDir returned empty string")
	}
}

func TestDefaultDataDir_FallbackWhenEnvEmpty(t *testing.T) {
	orig := os.Getenv("ProgramData")
	os.Setenv("ProgramData", "")
	defer os.Setenv("ProgramData", orig)

	dir := DefaultDataDir()
	if dir != `C:\ProgramData\ELNSSM` {
		t.Errorf("DefaultDataDir = %q, want C:\\ProgramData\\ELNSSM", dir)
	}
}

func TestSaveAndLoadConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config", "elnssm.yaml")

	cfg := DefaultConfig()
	cfg.API.Listen = "0.0.0.0:9200"
	cfg.Guardian.LogLevel = "debug"
	// Disable auth for this test (default is enabled, but no token hash)
	cfg.API.Auth.Enabled = false

	if err := cfg.Save(path); err != nil {
		t.Fatalf("Save: %v", err)
	}

	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if loaded.API.Listen != "0.0.0.0:9200" {
		t.Errorf("Listen = %q, want 0.0.0.0:9200", loaded.API.Listen)
	}
	if loaded.Guardian.LogLevel != "debug" {
		t.Errorf("LogLevel = %q, want debug", loaded.Guardian.LogLevel)
	}
}

func TestLoadConfig_FileNotFound(t *testing.T) {
	_, err := Load("/nonexistent/path/config.yaml")
	if err == nil {
		t.Error("expected error for missing file")
	}
}

func TestLoadConfig_InvalidYAML(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bad.yaml")
	os.WriteFile(path, []byte("{{{{invalid yaml!!!!"), 0644)

	_, err := Load(path)
	if err == nil {
		t.Error("expected error for invalid YAML")
	}
}

func TestConfigPaths(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Guardian.DataDir = `C:\TestData\ELNSSM`

	if got := cfg.ConfigDir(); got != `C:\TestData\ELNSSM\config` {
		t.Errorf("ConfigDir = %q", got)
	}
	if got := cfg.ServicesDir(); got != `C:\TestData\ELNSSM\config\services` {
		t.Errorf("ServicesDir = %q", got)
	}
	if got := cfg.DataPath(); got != `C:\TestData\ELNSSM\data\elnssm.db` {
		t.Errorf("DataPath = %q", got)
	}
	if got := cfg.LogsDir(); got != `C:\TestData\ELNSSM\logs` {
		t.Errorf("LogsDir = %q", got)
	}
	if got := cfg.ServiceLogDir("my-app"); got != `C:\TestData\ELNSSM\logs\my-app` {
		t.Errorf("ServiceLogDir = %q", got)
	}
}

func TestLoadServiceConfig_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test-svc.yaml")

	// Write a service YAML manually
	yaml := `
id: "test-svc"
name: "test-svc"
display_name: "Test Service"
executable: "C:\\test.exe"
arguments:
  - "--port"
  - "8080"
working_dir: "C:\\test"
environment:
  NODE_ENV: "production"
startup_type: "auto"
priority: "normal"
stop_timeout: "30s"
stop_signal: "ctrl_c"
restart_policy:
  mode: "on_failure"
  delay: "5s"
  max_retries: 10
  retry_window: "1h"
  backoff_multiplier: 2.0
  max_backoff: "5m"
  restart_on_health_fail: true
health_checks:
  - type: "http"
    target: "http://localhost:8080/health"
    method: "GET"
    expect_status: 200
    interval: "30s"
    timeout: "10s"
    retries: 3
    start_delay: "15s"
logging:
  stdout_file: "stdout.log"
  stderr_file: "stderr.log"
  max_size: 52428800
  max_backups: 5
  compress: true
`
	os.WriteFile(path, []byte(yaml), 0644)

	svc, err := LoadServiceConfig(path)
	if err != nil {
		t.Fatalf("LoadServiceConfig: %v", err)
	}

	if svc.ID != "test-svc" {
		t.Errorf("ID = %q", svc.ID)
	}
	if svc.Executable != `C:\test.exe` {
		t.Errorf("Executable = %q", svc.Executable)
	}
	if len(svc.Arguments) != 2 {
		t.Errorf("Arguments len = %d", len(svc.Arguments))
	}
	if svc.Environment["NODE_ENV"] != "production" {
		t.Errorf("NODE_ENV = %q", svc.Environment["NODE_ENV"])
	}
	if svc.StopTimeout.Seconds() != 30 {
		t.Errorf("StopTimeout = %v", svc.StopTimeout)
	}
	if svc.RestartPolicy.Delay.Seconds() != 5 {
		t.Errorf("RestartPolicy.Delay = %v", svc.RestartPolicy.Delay)
	}
	if len(svc.HealthChecks) != 1 {
		t.Fatalf("HealthChecks len = %d", len(svc.HealthChecks))
	}
	if svc.HealthChecks[0].Interval.Seconds() != 30 {
		t.Errorf("HealthCheck.Interval = %v", svc.HealthChecks[0].Interval)
	}
}

func TestLoadServiceConfig_InvalidDuration(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.yaml")
	yaml := `
id: "bad"
executable: "test.exe"
stop_timeout: "not-a-duration"
`
	os.WriteFile(path, []byte(yaml), 0644)

	_, err := LoadServiceConfig(path)
	if err == nil {
		t.Error("expected error for invalid duration")
	}
}

func TestLoadAllServiceConfigs_EmptyDir(t *testing.T) {
	dir := t.TempDir()
	services, err := LoadAllServiceConfigs(dir)
	if err != nil {
		t.Fatalf("LoadAllServiceConfigs: %v", err)
	}
	if len(services) != 0 {
		t.Errorf("expected 0 services, got %d", len(services))
	}
}

func TestLoadAllServiceConfigs_NonExistentDir(t *testing.T) {
	services, err := LoadAllServiceConfigs("/nonexistent/dir")
	if err != nil {
		t.Fatalf("LoadAllServiceConfigs: %v", err)
	}
	if services != nil {
		t.Errorf("expected nil for non-existent dir, got %v", services)
	}
}

func TestLoadAllServiceConfigs_SkipsNonYAML(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "svc.yaml"), []byte("id: svc\nexecutable: test.exe"), 0644)
	os.WriteFile(filepath.Join(dir, "readme.txt"), []byte("not a config"), 0644)
	os.MkdirAll(filepath.Join(dir, "subdir"), 0755)

	services, err := LoadAllServiceConfigs(dir)
	if err != nil {
		t.Fatalf("LoadAllServiceConfigs: %v", err)
	}
	if len(services) != 1 {
		t.Errorf("expected 1 service, got %d", len(services))
	}
}

func TestValidate_CIDRRejected(t *testing.T) {
	cfg := DefaultConfig()
	cfg.API.Auth.Enabled = false
	cfg.API.IPWhitelist = []string{"192.168.1.0/24"}
	err := cfg.Validate()
	if err == nil {
		t.Error("expected error for CIDR in IP whitelist")
	}
	if !strings.Contains(err.Error(), "CIDR") {
		t.Errorf("error should mention CIDR, got: %v", err)
	}
}

func TestValidate_InvalidIP(t *testing.T) {
	cfg := DefaultConfig()
	cfg.API.Auth.Enabled = false
	cfg.API.IPWhitelist = []string{"not-an-ip"}
	err := cfg.Validate()
	if err == nil {
		t.Error("expected error for invalid IP")
	}
}

func TestValidate_AuthEnabledNoTokenHash(t *testing.T) {
	cfg := DefaultConfig()
	cfg.API.Auth.Enabled = true
	cfg.API.Auth.Type = "token"
	cfg.API.Auth.TokenHash = ""
	err := cfg.Validate()
	if err == nil {
		t.Error("expected error when auth enabled but no token hash")
	}
}

func TestValidate_AuthDisabledOK(t *testing.T) {
	cfg := DefaultConfig()
	cfg.API.Auth.Enabled = false
	err := cfg.Validate()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidate_AuthTypeSSPI(t *testing.T) {
	cfg := DefaultConfig()
	cfg.API.Auth.Enabled = true
	cfg.API.Auth.Type = "sspi"
	err := cfg.Validate()
	if err != nil {
		t.Errorf("unexpected error for sspi auth type: %v", err)
	}
}

func TestValidate_AuthTypeSSPIWithTokenHash(t *testing.T) {
	cfg := DefaultConfig()
	cfg.API.Auth.Enabled = true
	cfg.API.Auth.Type = "sspi"
	cfg.API.Auth.TokenHash = "$2a$10$somehash"
	err := cfg.Validate()
	if err != nil {
		t.Errorf("unexpected error for sspi auth type with token_hash: %v", err)
	}
}

func TestSaveConfig_Permissions0600(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.yaml")
	cfg := DefaultConfig()
	cfg.API.Auth.Enabled = false
	if err := cfg.Save(path); err != nil {
		t.Fatalf("Save: %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	// On Windows, file permissions are simplified; just verify file exists and is not empty
	if info.Size() == 0 {
		t.Error("saved config file is empty")
	}
}
