// Package logging captures stdout/stderr from managed services, rotates
// the resulting log files via lumberjack and streams new log lines to
// connected WebSocket clients in real time.
package logging

import (
	"bufio"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"sync"

	"gopkg.in/natefinch/lumberjack.v2"
)

// Capture manages stdout/stderr log capture for a service.
type Capture struct {
	serviceID string
	logDir    string
	config    CaptureConfig
	writers   []*lumberjack.Logger
	streamer  *Streamer
	mu        sync.Mutex
}

// CaptureConfig holds log capture settings.
type CaptureConfig struct {
	StdoutFile    string
	StderrFile    string
	CombineOutput bool
	MaxSizeMB     int
	MaxBackups    int
	Compress      bool
}

// NewCapture creates a new log capture instance.
func NewCapture(serviceID, logDir string, cfg CaptureConfig, streamer *Streamer) *Capture {
	return &Capture{
		serviceID: serviceID,
		logDir:    logDir,
		config:    cfg,
		streamer:  streamer,
	}
}

// Start begins capturing stdout and stderr from the given readers.
func (c *Capture) Start(stdout, stderr io.ReadCloser) {
	if err := os.MkdirAll(c.logDir, 0o750); err != nil {
		slog.Error("Failed to create log directory", "dir", c.logDir, "error", err)
		return
	}

	stdoutFile := c.config.StdoutFile
	if stdoutFile == "" {
		stdoutFile = "stdout.log"
	}
	stderrFile := c.config.StderrFile
	if stderrFile == "" {
		stderrFile = "stderr.log"
	}

	maxSize := c.config.MaxSizeMB
	if maxSize == 0 {
		maxSize = 50
	}

	stdoutLogger := &lumberjack.Logger{
		Filename:   filepath.Join(c.logDir, stdoutFile),
		MaxSize:    maxSize,
		MaxBackups: c.config.MaxBackups,
		Compress:   c.config.Compress,
	}

	c.mu.Lock()
	c.writers = append(c.writers, stdoutLogger)
	c.mu.Unlock()

	go c.captureStream("stdout", stdout, stdoutLogger)

	if c.config.CombineOutput {
		go c.captureStream("stderr", stderr, stdoutLogger)
	} else {
		stderrLogger := &lumberjack.Logger{
			Filename:   filepath.Join(c.logDir, stderrFile),
			MaxSize:    maxSize,
			MaxBackups: c.config.MaxBackups,
			Compress:   c.config.Compress,
		}
		c.mu.Lock()
		c.writers = append(c.writers, stderrLogger)
		c.mu.Unlock()
		go c.captureStream("stderr", stderr, stderrLogger)
	}
}

func (c *Capture) captureStream(stream string, reader io.ReadCloser, logger *lumberjack.Logger) {
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024) // 1MB line buffer

	for scanner.Scan() {
		line := scanner.Bytes()
		// Allocate a fresh slice to avoid aliasing the scanner's internal buffer.
		lineWithNewline := make([]byte, len(line)+1)
		copy(lineWithNewline, line)
		lineWithNewline[len(line)] = '\n'

		// Write to log file
		if _, err := logger.Write(lineWithNewline); err != nil {
			slog.Debug("Failed to write to log", "service", c.serviceID, "stream", stream, "error", err)
		}

		// Send to WebSocket streamer
		if c.streamer != nil {
			c.streamer.Broadcast(c.serviceID, stream, string(line))
		}
	}
}

// Close flushes and closes all log writers.
func (c *Capture) Close() {
	c.mu.Lock()
	defer c.mu.Unlock()

	for _, w := range c.writers {
		_ = w.Close()
	}
	c.writers = nil
}

// GetLogPath returns the path to a log file.
func (c *Capture) GetLogPath(stream string) string {
	switch stream {
	case "stderr":
		f := c.config.StderrFile
		if f == "" {
			f = "stderr.log"
		}
		return filepath.Join(c.logDir, f)
	default:
		f := c.config.StdoutFile
		if f == "" {
			f = "stdout.log"
		}
		return filepath.Join(c.logDir, f)
	}
}
