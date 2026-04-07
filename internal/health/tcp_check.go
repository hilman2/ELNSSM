package health

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"strings"
	"time"

	"github.com/hilman2/ELNSSM/internal/model"
)

// TCPChecker performs TCP port connectivity health checks.
type TCPChecker struct {
	cfg model.HealthCheckConfig
}

// NewTCPChecker creates a new TCP health checker.
func NewTCPChecker(cfg model.HealthCheckConfig) *TCPChecker {
	return &TCPChecker{cfg: cfg}
}

func (c *TCPChecker) Type() model.HealthCheckType {
	return model.HealthCheckTCP
}

func (c *TCPChecker) Check(ctx context.Context) model.HealthCheckResult {
	start := time.Now()

	timeout := c.cfg.Timeout
	if timeout == 0 {
		timeout = 5 * time.Second
	}

	dialer := &net.Dialer{Timeout: timeout}
	conn, err := dialer.DialContext(ctx, "tcp", c.cfg.Target)
	duration := time.Since(start)

	if err != nil {
		return model.HealthCheckResult{
			CheckType: model.HealthCheckTCP,
			Status:    model.HealthStatusUnhealthy,
			Timestamp: start,
			Duration:  duration,
			Message:   fmt.Sprintf("connection failed: %v", err),
		}
	}
	defer conn.Close()

	// If no send/expect configured, connectivity check is sufficient
	if c.cfg.Send == "" && c.cfg.ExpectResp == "" {
		return model.HealthCheckResult{
			CheckType: model.HealthCheckTCP,
			Status:    model.HealthStatusHealthy,
			Timestamp: start,
			Duration:  time.Since(start),
			Message:   fmt.Sprintf("TCP connection to %s successful", c.cfg.Target),
		}
	}

	// Set deadline for send/receive operations
	if err := conn.SetDeadline(time.Now().Add(timeout)); err != nil {
		return model.HealthCheckResult{
			CheckType: model.HealthCheckTCP,
			Status:    model.HealthStatusUnhealthy,
			Timestamp: start,
			Duration:  time.Since(start),
			Message:   fmt.Sprintf("set deadline failed: %v", err),
		}
	}

	// Send data if configured
	if c.cfg.Send != "" {
		data := c.cfg.Send + "\r\n"
		if _, err := conn.Write([]byte(data)); err != nil {
			return model.HealthCheckResult{
				CheckType: model.HealthCheckTCP,
				Status:    model.HealthStatusUnhealthy,
				Timestamp: start,
				Duration:  time.Since(start),
				Message:   fmt.Sprintf("send failed: %v", err),
			}
		}
	}

	// Read and check response if configured
	if c.cfg.ExpectResp != "" {
		buf := make([]byte, 4096)
		n, err := conn.Read(buf)
		if err != nil && !errors.Is(err, io.EOF) {
			return model.HealthCheckResult{
				CheckType: model.HealthCheckTCP,
				Status:    model.HealthStatusUnhealthy,
				Timestamp: start,
				Duration:  time.Since(start),
				Message:   fmt.Sprintf("read failed: %v", err),
			}
		}
		response := string(buf[:n])
		if !strings.Contains(response, c.cfg.ExpectResp) {
			return model.HealthCheckResult{
				CheckType: model.HealthCheckTCP,
				Status:    model.HealthStatusUnhealthy,
				Timestamp: start,
				Duration:  time.Since(start),
				Message:   fmt.Sprintf("expected %q in response, got %q", c.cfg.ExpectResp, truncate(response, 200)),
			}
		}
	}

	return model.HealthCheckResult{
		CheckType: model.HealthCheckTCP,
		Status:    model.HealthStatusHealthy,
		Timestamp: start,
		Duration:  time.Since(start),
		Message:   fmt.Sprintf("TCP check to %s passed", c.cfg.Target),
	}
}
