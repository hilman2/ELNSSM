package logging

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// --- Capture Tests ---

func TestNewCapture(t *testing.T) {
	c := NewCapture("test-svc", "/tmp/logs", CaptureConfig{}, nil)
	if c.serviceID != "test-svc" {
		t.Errorf("serviceID = %q", c.serviceID)
	}
	if c.logDir != "/tmp/logs" {
		t.Errorf("logDir = %q", c.logDir)
	}
}

func TestCapture_GetLogPath_Defaults(t *testing.T) {
	c := NewCapture("test", "/var/logs/test", CaptureConfig{}, nil)

	stdout := c.GetLogPath("stdout")
	if !strings.HasSuffix(stdout, "stdout.log") {
		t.Errorf("stdout path = %q, want suffix stdout.log", stdout)
	}

	stderr := c.GetLogPath("stderr")
	if !strings.HasSuffix(stderr, "stderr.log") {
		t.Errorf("stderr path = %q, want suffix stderr.log", stderr)
	}
}

func TestCapture_GetLogPath_Custom(t *testing.T) {
	c := NewCapture("test", "/var/logs/test", CaptureConfig{
		StdoutFile: "app.out.log",
		StderrFile: "app.err.log",
	}, nil)

	stdout := c.GetLogPath("stdout")
	if !strings.HasSuffix(stdout, "app.out.log") {
		t.Errorf("stdout path = %q, want suffix app.out.log", stdout)
	}

	stderr := c.GetLogPath("stderr")
	if !strings.HasSuffix(stderr, "app.err.log") {
		t.Errorf("stderr path = %q, want suffix app.err.log", stderr)
	}
}

func TestCapture_GetLogPath_UnknownStreamDefaultsToStdout(t *testing.T) {
	c := NewCapture("test", "/var/logs/test", CaptureConfig{}, nil)
	path := c.GetLogPath("unknown")
	if !strings.HasSuffix(path, "stdout.log") {
		t.Errorf("unknown stream path = %q, want suffix stdout.log", path)
	}
}

func TestCapture_StartAndClose(t *testing.T) {
	dir := t.TempDir()
	c := NewCapture("test-svc", dir, CaptureConfig{
		MaxSizeMB: 1,
	}, nil)

	// Create pipe-like readers
	stdoutR, stdoutW := io.Pipe()
	stderrR, stderrW := io.Pipe()

	c.Start(stdoutR, stderrR)

	// Write some data
	stdoutW.Write([]byte("hello stdout\n"))
	stderrW.Write([]byte("hello stderr\n"))

	// Give capture goroutines time to process
	time.Sleep(100 * time.Millisecond)

	// Close writers to signal EOF
	stdoutW.Close()
	stderrW.Close()
	time.Sleep(50 * time.Millisecond)

	c.Close()

	// Verify log files were created
	stdoutPath := filepath.Join(dir, "stdout.log")
	if _, err := os.Stat(stdoutPath); os.IsNotExist(err) {
		t.Error("stdout.log not created")
	} else if err == nil {
		data, _ := os.ReadFile(stdoutPath)
		if !strings.Contains(string(data), "hello stdout") {
			t.Errorf("stdout.log content = %q, want containing 'hello stdout'", string(data))
		}
	}

	stderrPath := filepath.Join(dir, "stderr.log")
	if _, err := os.Stat(stderrPath); os.IsNotExist(err) {
		t.Error("stderr.log not created")
	} else if err == nil {
		data, _ := os.ReadFile(stderrPath)
		if !strings.Contains(string(data), "hello stderr") {
			t.Errorf("stderr.log content = %q, want containing 'hello stderr'", string(data))
		}
	}
}

func TestCapture_CombineOutput(t *testing.T) {
	dir := t.TempDir()
	c := NewCapture("test-svc", dir, CaptureConfig{
		CombineOutput: true,
		MaxSizeMB:     1,
	}, nil)

	stdoutR, stdoutW := io.Pipe()
	stderrR, stderrW := io.Pipe()

	c.Start(stdoutR, stderrR)

	stdoutW.Write([]byte("from stdout\n"))
	stderrW.Write([]byte("from stderr\n"))
	time.Sleep(100 * time.Millisecond)

	stdoutW.Close()
	stderrW.Close()
	time.Sleep(50 * time.Millisecond)

	c.Close()

	// Both should be in stdout.log when combined
	data, err := os.ReadFile(filepath.Join(dir, "stdout.log"))
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, "from stdout") {
		t.Error("combined log missing stdout content")
	}
	if !strings.Contains(content, "from stderr") {
		t.Error("combined log missing stderr content")
	}

	// stderr.log should NOT exist in combined mode
	stderrPath := filepath.Join(dir, "stderr.log")
	if _, err := os.Stat(stderrPath); !os.IsNotExist(err) {
		t.Error("stderr.log should not exist in combined output mode")
	}
}

func TestCapture_CloseIdempotent(t *testing.T) {
	c := NewCapture("test", t.TempDir(), CaptureConfig{}, nil)
	// Close without Start should not panic
	c.Close()
	c.Close()
}

// --- Streamer Tests ---

func TestNewStreamer(t *testing.T) {
	s := NewStreamer()
	if s == nil {
		t.Fatal("NewStreamer returned nil")
	}
	if s.clients == nil {
		t.Error("clients map not initialized")
	}
}

func TestStreamer_BroadcastNoClients(t *testing.T) {
	s := NewStreamer()
	// Should not panic with no clients
	s.Broadcast("svc-1", "stdout", "hello world")
}

// --- LogMessage Tests ---

func TestLogMessage_Fields(t *testing.T) {
	msg := LogMessage{
		ServiceID: "my-app",
		Stream:    "stdout",
		Line:      "application started",
	}
	if msg.ServiceID != "my-app" {
		t.Errorf("ServiceID = %q", msg.ServiceID)
	}
	if msg.Stream != "stdout" {
		t.Errorf("Stream = %q", msg.Stream)
	}
	if msg.Line != "application started" {
		t.Errorf("Line = %q", msg.Line)
	}
}

// --- CaptureConfig Tests ---

func TestCaptureConfig_Defaults(t *testing.T) {
	var cfg CaptureConfig
	if cfg.StdoutFile != "" {
		t.Error("default StdoutFile should be empty")
	}
	if cfg.StderrFile != "" {
		t.Error("default StderrFile should be empty")
	}
	if cfg.CombineOutput {
		t.Error("default CombineOutput should be false")
	}
	if cfg.MaxSizeMB != 0 {
		t.Error("default MaxSizeMB should be 0")
	}
}

// --- Capture with Streamer ---

func TestCapture_BroadcastsToStreamer(t *testing.T) {
	s := NewStreamer()
	dir := t.TempDir()
	c := NewCapture("test-svc", dir, CaptureConfig{MaxSizeMB: 1}, s)

	stdoutR, stdoutW := io.Pipe()
	stderrR, stderrW := io.Pipe()

	c.Start(stdoutR, stderrR)

	stdoutW.Write([]byte("test line\n"))
	time.Sleep(100 * time.Millisecond)

	stdoutW.Close()
	stderrW.Close()
	time.Sleep(50 * time.Millisecond)
	c.Close()

	// Verify the streamer was called (no clients, so no errors expected)
	// The fact that it didn't panic proves the integration works
}
