package process

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
		defer func() { _ = os.Remove(tmpFile) }()

		// Lifecycle hook scripts are explicitly user-configured;
		// running them is the entire point of this code path.
		if strings.HasSuffix(tmpFile, ".ps1") {
			cmd = exec.CommandContext(ctx, "powershell", "-ExecutionPolicy", "Bypass", "-File", tmpFile) //nolint:gosec // user-configured lifecycle hook
		} else {
			cmd = exec.CommandContext(ctx, "cmd", "/C", tmpFile) //nolint:gosec // user-configured lifecycle hook
		}
	} else if hook.Command != "" {
		cmd = exec.CommandContext(ctx, hook.Command, hook.Args...) //nolint:gosec // user-configured lifecycle hook
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

// writeTempScript writes a hook's script body to a temp file and returns its
// path, choosing the extension from the script's apparent language. The caller
// removes the file.
//
// Creation has to be exclusive and the name unpredictable. Hooks run with the
// Guardian's privileges, normally LocalSystem, which puts os.TempDir() at
// C:\Windows\Temp where unprivileged users may also create files. A name built
// from a timestamp can be guessed, letting someone place the file first or
// swap it between the write and the execution and have it run as SYSTEM.
// os.CreateTemp gives an unguessable name and O_EXCL; the 0o600 mode used
// before did nothing here, because on Windows the file takes the directory's
// ACL. The health package writes its check scripts the same way.
func writeTempScript(body string) (string, error) {
	ext := ".cmd"
	if isPowerShellScript(body) {
		ext = ".ps1"
	}

	f, err := os.CreateTemp("", "elnssm-hook-*"+ext)
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

// hookCmdletRe matches a PowerShell Verb-Noun cmdlet such as Stop-Service.
// Batch has no such construct, which makes it a stronger signal than a list of
// keywords: "Stop-Service Spooler" contains none of the keywords used before
// and so ran under cmd.exe, where it could only fail.
var hookCmdletRe = regexp.MustCompile(`\b[A-Z][a-zA-Z]+-[A-Z][a-zA-Z]+\b`)

// isPowerShellScript reports whether the body carries syntax only PowerShell
// has. Unlike a health check, a lifecycle hook has no field to state its
// interpreter, so this guess is all there is.
func isPowerShellScript(body string) bool {
	if hookCmdletRe.MatchString(body) {
		return true
	}
	for _, marker := range []string{"$", "param(", "-eq ", "-ne ", "-match ", "-gt ", "-lt ", "Import-Module"} {
		if strings.Contains(body, marker) {
			return true
		}
	}
	return false
}
