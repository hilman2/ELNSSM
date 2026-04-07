// Package winservice provides a thin wrapper over the Windows Service
// Control Manager (SCM) for listing and controlling native Windows
// services from the ELNSSM API.
package winservice

import (
	"fmt"
	"time"

	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/mgr"
)

// NativeService represents a Windows SCM service.
type NativeService struct {
	Name        string `json:"name"`
	DisplayName string `json:"display_name"`
	Description string `json:"description"`
	Status      string `json:"status"`
	StartType   string `json:"start_type"`
	BinaryPath  string `json:"binary_path"`
	Account     string `json:"account"`
	PID         uint32 `json:"pid"`
}

// ListNativeServices returns all Windows services from the SCM.
func ListNativeServices() ([]NativeService, error) {
	m, err := mgr.Connect()
	if err != nil {
		return nil, fmt.Errorf("connecting to SCM: %w", err)
	}
	defer func() { _ = m.Disconnect() }()

	names, err := m.ListServices()
	if err != nil {
		return nil, fmt.Errorf("listing services: %w", err)
	}

	services := make([]NativeService, 0, len(names))
	for _, name := range names {
		svcInfo, err := getServiceInfo(m, name)
		if err != nil {
			continue // skip services we can't read
		}
		services = append(services, *svcInfo)
	}

	return services, nil
}

// GetNativeService returns info about a single Windows service.
func GetNativeService(name string) (*NativeService, error) {
	m, err := mgr.Connect()
	if err != nil {
		return nil, fmt.Errorf("connecting to SCM: %w", err)
	}
	defer func() { _ = m.Disconnect() }()

	return getServiceInfo(m, name)
}

// StartNativeService starts a Windows service.
func StartNativeService(name string) error {
	m, err := mgr.Connect()
	if err != nil {
		return fmt.Errorf("connecting to SCM: %w", err)
	}
	defer func() { _ = m.Disconnect() }()

	s, err := m.OpenService(name)
	if err != nil {
		return fmt.Errorf("opening service %q: %w", name, err)
	}
	defer func() { _ = s.Close() }()

	return s.Start()
}

// StopNativeService stops a Windows service.
func StopNativeService(name string) error {
	m, err := mgr.Connect()
	if err != nil {
		return fmt.Errorf("connecting to SCM: %w", err)
	}
	defer func() { _ = m.Disconnect() }()

	s, err := m.OpenService(name)
	if err != nil {
		return fmt.Errorf("opening service %q: %w", name, err)
	}
	defer func() { _ = s.Close() }()

	_, err = s.Control(svc.Stop)
	if err != nil {
		return fmt.Errorf("stopping service %q: %w", name, err)
	}

	return nil
}

// RestartNativeService stops and then starts a Windows service.
func RestartNativeService(name string) error {
	if err := StopNativeService(name); err != nil {
		return err
	}

	// Wait for the service to stop
	m, err := mgr.Connect()
	if err != nil {
		return fmt.Errorf("connecting to SCM: %w", err)
	}
	defer func() { _ = m.Disconnect() }()

	s, err := m.OpenService(name)
	if err != nil {
		return fmt.Errorf("opening service %q: %w", name, err)
	}
	defer func() { _ = s.Close() }()

	// Poll for stopped state (max 30s)
	for i := 0; i < 60; i++ {
		status, err := s.Query()
		if err != nil {
			break
		}
		if status.State == svc.Stopped {
			break
		}
		time.Sleep(500 * time.Millisecond)
	}

	return s.Start()
}

func getServiceInfo(m *mgr.Mgr, name string) (*NativeService, error) {
	s, err := m.OpenService(name)
	if err != nil {
		return nil, fmt.Errorf("opening service %q: %w", name, err)
	}
	defer func() { _ = s.Close() }()

	cfg, err := s.Config()
	if err != nil {
		return nil, fmt.Errorf("reading config for %q: %w", name, err)
	}

	status, err := s.Query()
	if err != nil {
		return nil, fmt.Errorf("querying status for %q: %w", name, err)
	}

	return &NativeService{
		Name:        name,
		DisplayName: cfg.DisplayName,
		Description: cfg.Description,
		Status:      stateToString(status.State),
		StartType:   startTypeToString(cfg.StartType),
		BinaryPath:  cfg.BinaryPathName,
		Account:     cfg.ServiceStartName,
		PID:         status.ProcessId,
	}, nil
}

func stateToString(state svc.State) string {
	switch state {
	case svc.Stopped:
		return "stopped"
	case svc.StartPending:
		return "start_pending"
	case svc.StopPending:
		return "stop_pending"
	case svc.Running:
		return "running"
	case svc.ContinuePending:
		return "continue_pending"
	case svc.PausePending:
		return "pause_pending"
	case svc.Paused:
		return "paused"
	default:
		return "unknown"
	}
}

func startTypeToString(startType uint32) string {
	switch startType {
	case mgr.StartAutomatic:
		return "automatic"
	case mgr.StartManual:
		return "manual"
	case mgr.StartDisabled:
		return "disabled"
	default:
		return "unknown"
	}
}
