package health

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/hilman2/ELNSSM/internal/model"
)

// HTTPChecker performs HTTP health checks.
type HTTPChecker struct {
	cfg    model.HealthCheckConfig
	client *http.Client
}

// NewHTTPChecker creates a new HTTP health checker.
func NewHTTPChecker(cfg model.HealthCheckConfig) *HTTPChecker {
	return &HTTPChecker{
		cfg: cfg,
		client: &http.Client{
			Timeout: cfg.Timeout,
		},
	}
}

func (c *HTTPChecker) Type() model.HealthCheckType {
	return model.HealthCheckHTTP
}

func (c *HTTPChecker) Check(ctx context.Context) model.HealthCheckResult {
	start := time.Now()

	method := c.cfg.Method
	if method == "" {
		method = "GET"
	}

	req, err := http.NewRequestWithContext(ctx, method, c.cfg.Target, nil)
	if err != nil {
		return model.HealthCheckResult{
			CheckType: model.HealthCheckHTTP,
			Status:    model.HealthStatusUnhealthy,
			Timestamp: start,
			Duration:  time.Since(start),
			Message:   fmt.Sprintf("creating request: %v", err),
		}
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return model.HealthCheckResult{
			CheckType: model.HealthCheckHTTP,
			Status:    model.HealthStatusUnhealthy,
			Timestamp: start,
			Duration:  time.Since(start),
			Message:   fmt.Sprintf("request failed: %v", err),
		}
	}
	defer resp.Body.Close()

	result := model.HealthCheckResult{
		CheckType:  model.HealthCheckHTTP,
		Timestamp:  start,
		Duration:   time.Since(start),
		StatusCode: resp.StatusCode,
	}

	// Check status code
	expectedStatus := c.cfg.ExpectStatus
	if expectedStatus == 0 {
		expectedStatus = 200
	}
	if resp.StatusCode != expectedStatus {
		result.Status = model.HealthStatusUnhealthy
		result.Message = fmt.Sprintf("expected status %d, got %d", expectedStatus, resp.StatusCode)
		return result
	}

	// Check body content if configured
	if c.cfg.ExpectBody != "" {
		body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20)) // 1MB limit
		if err != nil {
			result.Status = model.HealthStatusUnhealthy
			result.Message = fmt.Sprintf("reading body: %v", err)
			return result
		}
		if !strings.Contains(string(body), c.cfg.ExpectBody) {
			result.Status = model.HealthStatusUnhealthy
			result.Message = fmt.Sprintf("body does not contain expected string %q", c.cfg.ExpectBody)
			return result
		}
	}

	result.Status = model.HealthStatusHealthy
	result.Message = fmt.Sprintf("HTTP %d OK", resp.StatusCode)
	return result
}
