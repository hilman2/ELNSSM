package health

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestScriptShellHonoursExplicitInterpreter(t *testing.T) {
	tests := []struct {
		interpreter string
		body        string
		want        Shell
	}{
		// An explicit value wins even when the body looks like the other one.
		{"cmd", "Get-Service Spooler", ShellCmd},
		{"powershell", "echo hello", ShellPowerShell},
		{"PowerShell", "echo hello", ShellPowerShell},
		{"  pwsh  ", "echo hello", ShellPowerShell},
		{"bat", "$x = 1", ShellCmd},
	}

	for _, tt := range tests {
		t.Run(tt.interpreter, func(t *testing.T) {
			if got := ScriptShell(tt.interpreter, tt.body); got != tt.want {
				t.Errorf("ScriptShell(%q, %q) = %q, want %q", tt.interpreter, tt.body, got, tt.want)
			}
		})
	}
}

// Without an explicit interpreter the shell is guessed. "Stop-Service Spooler"
// is the case the old keyword list missed: it matched none of its keywords, so
// the script ran under cmd.exe and failed on every single check.
func TestScriptShellGuessesFromBody(t *testing.T) {
	tests := []struct {
		name string
		body string
		want Shell
	}{
		{"verb-noun cmdlet", "Stop-Service Spooler", ShellPowerShell},
		{"verb-noun cmdlet, multi word noun", "Restart-Computer -Force", ShellPowerShell},
		{"get cmdlet", "Get-Service Spooler", ShellPowerShell},
		{"variable", "$svc = 1", ShellPowerShell},
		{"comparison operator", "if ($a -eq 1) { exit 0 }", ShellPowerShell},
		{"param block", "param($name)", ShellPowerShell},
		{"plain batch", "echo hello", ShellCmd},
		{"batch with errorlevel", "ping -n 1 localhost\r\nif errorlevel 1 exit 1", ShellCmd},
		{"empty", "", ShellCmd},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ScriptShell("", tt.body); got != tt.want {
				t.Errorf("ScriptShell(\"\", %q) = %q, want %q", tt.body, got, tt.want)
			}
		})
	}
}

func TestWriteTempScriptExtension(t *testing.T) {
	tests := []struct {
		shell   Shell
		wantExt string
	}{
		{ShellPowerShell, ".ps1"},
		{ShellCmd, ".cmd"},
	}

	for _, tt := range tests {
		t.Run(string(tt.shell), func(t *testing.T) {
			path, err := writeTempScript("echo hello", tt.shell)
			if err != nil {
				t.Fatalf("writeTempScript: %v", err)
			}
			defer func() { _ = os.Remove(path) }()

			if filepath.Ext(path) != tt.wantExt {
				t.Errorf("extension = %q, want %q", filepath.Ext(path), tt.wantExt)
			}

			content, err := os.ReadFile(path) //nolint:gosec // path came from os.CreateTemp
			if err != nil {
				t.Fatalf("reading back the script: %v", err)
			}
			if string(content) != "echo hello" {
				t.Errorf("content = %q, want %q", content, "echo hello")
			}
		})
	}
}

// The old implementation derived the name from a timestamp, which let a local
// user predict it and place or swap the file before the Guardian ran it as
// SYSTEM. Distinct names on consecutive calls is the observable part of using
// os.CreateTemp, which also creates the file exclusively.
func TestWriteTempScriptNamesAreUnique(t *testing.T) {
	seen := make(map[string]bool)
	t.Cleanup(func() {
		for path := range seen {
			_ = os.Remove(path)
		}
	})

	for i := 0; i < 20; i++ {
		path, err := writeTempScript("echo hello", ShellCmd)
		if err != nil {
			t.Fatalf("writeTempScript: %v", err)
		}

		if seen[path] {
			t.Fatalf("writeTempScript reused the path %q", path)
		}
		seen[path] = true

		if !strings.Contains(filepath.Base(path), "elnssm-hc-") {
			t.Errorf("name %q lost the elnssm-hc- prefix", filepath.Base(path))
		}
	}
}
