package health

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"regexp"
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
		shell := ScriptShell(c.cfg.Interpreter, c.cfg.ScriptBody)

		// Inline script: write to temp file and execute
		tmpFile, err := writeTempScript(c.cfg.ScriptBody, shell)
		if err != nil {
			return model.HealthCheckResult{
				CheckType: model.HealthCheckScript,
				Status:    model.HealthStatusUnhealthy,
				Timestamp: start,
				Duration:  time.Since(start),
				Message:   fmt.Sprintf("failed to write temp script: %v", err),
			}
		}
		defer func() { _ = os.Remove(tmpFile) }()

		// Health check scripts are explicitly user-configured;
		// this whole feature exists to run them.
		if shell == ShellPowerShell {
			cmd = exec.CommandContext(ctx, "powershell", "-ExecutionPolicy", "Bypass", "-File", tmpFile) //nolint:gosec // user-configured health check
		} else {
			cmd = exec.CommandContext(ctx, "cmd", "/C", tmpFile) //nolint:gosec // user-configured health check
		}
	} else {
		// Legacy: single command in Target field, also user-configured.
		cmd = exec.CommandContext(ctx, "cmd", "/C", c.cfg.Target) //nolint:gosec // user-configured health check
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

// Shell identifies which interpreter runs an inline health check script.
type Shell string

const (
	ShellCmd        Shell = "cmd"
	ShellPowerShell Shell = "powershell"
)

// cmdletRe matches a PowerShell Verb-Noun cmdlet such as Stop-Service or
// Restart-Computer: two capitalised words joined by a hyphen. Batch files have
// no such construct, which makes this a far better signal than a keyword list.
var cmdletRe = regexp.MustCompile(`\b[A-Z][a-zA-Z]+-[A-Z][a-zA-Z]+\b`)

// ScriptShell decides which interpreter to use for a script body.
//
// interpreter is the value configured on the health check. "cmd" and
// "powershell" are honoured as given; anything else, including the empty
// string, falls through to guessing from the body.
//
// The guess exists because the interpreter field was added later and configs
// written before it carry no value. It cannot be reliable: a PowerShell script
// and a batch file have no marker that tells them apart, and picking wrong
// makes the check fail permanently, which with restart_on_health_fail turns
// into an endless restart loop. Set the field and the guess never runs.
func ScriptShell(interpreter, body string) Shell {
	switch strings.ToLower(strings.TrimSpace(interpreter)) {
	case string(ShellPowerShell), "pwsh", "ps1":
		return ShellPowerShell
	case string(ShellCmd), "batch", "bat":
		return ShellCmd
	}

	if looksLikePowerShell(body) {
		return ShellPowerShell
	}
	return ShellCmd
}

// looksLikePowerShell reports whether the body carries syntax that only
// PowerShell has: a Verb-Noun cmdlet, a $variable, a comparison operator such
// as -eq, or a param( block.
func looksLikePowerShell(body string) bool {
	if cmdletRe.MatchString(body) {
		return true
	}
	for _, marker := range []string{"$", "param(", "-eq ", "-ne ", "-match ", "-gt ", "-lt ", "Import-Module"} {
		if strings.Contains(body, marker) {
			return true
		}
	}
	return false
}

// writeTempScript writes the script body to a temporary file and returns its
// path. The caller is responsible for removing it.
//
// The name has to be unpredictable and the file has to be created exclusively.
// The Guardian usually runs as LocalSystem, so os.TempDir() is C:\Windows\Temp,
// where unprivileged users may also create files; a predictable name would let
// one of them place the file first, or swap it between the write and the
// execution, and have their content run as SYSTEM. os.CreateTemp gives both
// properties. Note that the 0o600 mode is nearly meaningless on Windows, where
// the file inherits the directory ACL, so exclusive creation is what carries
// this.
func writeTempScript(body string, shell Shell) (string, error) {
	ext := ".cmd"
	if shell == ShellPowerShell {
		ext = ".ps1"
	}

	f, err := os.CreateTemp("", "elnssm-hc-*"+ext)
	if err != nil {
		return "", err
	}
	name := f.Name()

	if _, err := f.WriteString(body); err != nil {
		f.Close()
		_ = os.Remove(name)
		return "", err
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(name)
		return "", err
	}
	return name, nil
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
