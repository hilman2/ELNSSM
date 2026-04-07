package health

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/hilman2/ELNSSM/internal/model"
)

// ScriptChecker executes a custom script/command for health checking.
type ScriptChecker struct {
	cfg model.HealthCheckConfig
}

// NewScriptChecker creates a new script health checker.
func NewScriptChecker(cfg model.HealthCheckConfig) *ScriptChecker {
	return &ScriptChecker{cfg: cfg}
}

func (c *ScriptChecker) Type() model.HealthCheckType {
	return model.HealthCheckScript
}

func (c *ScriptChecker) Check(ctx context.Context) model.HealthCheckResult {
	start := time.Now()

	timeout := c.cfg.Timeout
	if timeout == 0 {
		timeout = 30 * time.Second
	}

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	var cmd *exec.Cmd

	if c.cfg.ScriptBody != "" {
		// Inline script: write to temp file and execute
		tmpFile, err := c.writeTempScript(c.cfg.ScriptBody)
		if err != nil {
			return model.HealthCheckResult{
				CheckType: model.HealthCheckScript,
				Status:    model.HealthStatusUnhealthy,
				Timestamp: start,
				Duration:  time.Since(start),
				Message:   fmt.Sprintf("failed to write temp script: %v", err),
			}
		}
		defer os.Remove(tmpFile)

		if strings.HasSuffix(tmpFile, ".ps1") {
			cmd = exec.CommandContext(ctx, "powershell", "-ExecutionPolicy", "Bypass", "-File", tmpFile)
		} else {
			cmd = exec.CommandContext(ctx, "cmd", "/C", tmpFile)
		}
	} else {
		// Legacy: single command in Target field
		cmd = exec.CommandContext(ctx, "cmd", "/C", c.cfg.Target)
	}

	output, err := cmd.CombinedOutput()
	duration := time.Since(start)

	if err != nil {
		return model.HealthCheckResult{
			CheckType: model.HealthCheckScript,
			Status:    model.HealthStatusUnhealthy,
			Timestamp: start,
			Duration:  duration,
			Message:   fmt.Sprintf("script failed: %v (output: %s)", err, truncate(string(output), 500)),
		}
	}

	return model.HealthCheckResult{
		CheckType: model.HealthCheckScript,
		Status:    model.HealthStatusHealthy,
		Timestamp: start,
		Duration:  duration,
		Message:   fmt.Sprintf("script passed (output: %s)", truncate(string(output), 200)),
	}
}

// writeTempScript writes the script body to a temporary file and returns its path.
// Detects PowerShell content and uses .ps1 extension, otherwise .cmd.
func (c *ScriptChecker) writeTempScript(body string) (string, error) {
	ext := ".cmd"
	if isPowerShell(body) {
		ext = ".ps1"
	}

	tmpFile := filepath.Join(os.TempDir(), fmt.Sprintf("elnssm-hc-%d%s", time.Now().UnixNano(), ext))
	if err := os.WriteFile(tmpFile, []byte(body), 0600); err != nil {
		return "", err
	}
	return tmpFile, nil
}

// isPowerShell detects if the script body looks like PowerShell.
func isPowerShell(body string) bool {
	psKeywords := []string{"$", "Get-", "Set-", "Write-", "Invoke-", "param(", "Test-", "New-", "Import-Module", "foreach", "-eq", "-ne", "-match"}
	for _, kw := range psKeywords {
		if strings.Contains(body, kw) {
			return true
		}
	}
	return false
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
