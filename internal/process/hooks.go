package process

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

// ExecuteHook runs a lifecycle hook (command or inline script).
// Returns the combined output and any error.
func ExecuteHook(ctx context.Context, hook *model.LifecycleHook, env map[string]string) (string, error) {
	if hook == nil {
		return "", nil
	}

	timeout := hook.Timeout
	if timeout <= 0 {
		timeout = 30 * time.Second
	}

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	var cmd *exec.Cmd

	if hook.Script != "" {
		// Inline script: write to temp file and execute
		tmpFile, err := writeTempScript(hook.Script)
		if err != nil {
			return "", fmt.Errorf("writing temp script: %w", err)
		}
		defer os.Remove(tmpFile)

		if strings.HasSuffix(tmpFile, ".ps1") {
			cmd = exec.CommandContext(ctx, "powershell", "-ExecutionPolicy", "Bypass", "-File", tmpFile)
		} else {
			cmd = exec.CommandContext(ctx, "cmd", "/C", tmpFile)
		}
	} else if hook.Command != "" {
		cmd = exec.CommandContext(ctx, hook.Command, hook.Args...)
	} else {
		return "", nil // No command or script configured
	}

	// Set environment variables
	if len(env) > 0 {
		cmd.Env = os.Environ()
		for k, v := range env {
			cmd.Env = append(cmd.Env, k+"="+v)
		}
	}

	output, err := cmd.CombinedOutput()
	return string(output), err
}

// writeTempScript writes script body to a temp file, auto-detecting PS vs CMD.
func writeTempScript(body string) (string, error) {
	ext := ".cmd"
	if isPowerShellScript(body) {
		ext = ".ps1"
	}

	tmpFile := filepath.Join(os.TempDir(), fmt.Sprintf("elnssm-hook-%d%s", time.Now().UnixNano(), ext))
	if err := os.WriteFile(tmpFile, []byte(body), 0600); err != nil {
		return "", err
	}
	return tmpFile, nil
}

// isPowerShellScript detects if the script body looks like PowerShell.
func isPowerShellScript(body string) bool {
	psKeywords := []string{"$", "Get-", "Set-", "Write-", "Invoke-", "param(", "Test-", "New-", "Import-Module", "foreach", "-eq", "-ne", "-match"}
	for _, kw := range psKeywords {
		if strings.Contains(body, kw) {
			return true
		}
	}
	return false
}
