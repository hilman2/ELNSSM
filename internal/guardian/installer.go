package guardian

import (
	"fmt"
	"os"

	"golang.org/x/sys/windows/svc/eventlog"
	"golang.org/x/sys/windows/svc/mgr"
)

// Install registers the ELNSSM Guardian as a Windows service.
func Install() error {
	exePath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("getting executable path: %w", err)
	}

	m, err := mgr.Connect()
	if err != nil {
		return fmt.Errorf("connecting to service manager (are you running as administrator?): %w", err)
	}
	defer m.Disconnect()

	// Check if already installed
	s, err := m.OpenService(serviceName)
	if err == nil {
		s.Close()
		return fmt.Errorf("service %q is already installed", serviceName)
	}

	s, err = m.CreateService(serviceName, exePath, mgr.Config{
		DisplayName:  "ELNSSM Guardian",
		Description:  "ELNSSM Service Manager - manages child processes as Windows services",
		StartType:    mgr.StartAutomatic,
		ErrorControl: mgr.ErrorNormal,
	})
	if err != nil {
		return fmt.Errorf("creating service: %w", err)
	}
	defer s.Close()

	// Set recovery actions: restart on failure
	recoveryActions := []mgr.RecoveryAction{
		{Type: mgr.ServiceRestart, Delay: 5000},  // restart after 5s on 1st failure
		{Type: mgr.ServiceRestart, Delay: 10000}, // restart after 10s on 2nd failure
		{Type: mgr.ServiceRestart, Delay: 30000}, // restart after 30s on subsequent
	}
	if err := s.SetRecoveryActions(recoveryActions, 86400); err != nil {
		// Non-fatal: recovery actions are nice but not essential
		fmt.Printf("Warning: could not set recovery actions: %v\n", err)
	}

	// Install event log source
	if err := eventlog.InstallAsEventCreate(serviceName, eventlog.Error|eventlog.Warning|eventlog.Info); err != nil {
		fmt.Printf("Warning: could not install event log source: %v\n", err)
	}

	return nil
}

// Uninstall removes the ELNSSM Guardian Windows service.
func Uninstall() error {
	m, err := mgr.Connect()
	if err != nil {
		return fmt.Errorf("connecting to service manager (are you running as administrator?): %w", err)
	}
	defer m.Disconnect()

	s, err := m.OpenService(serviceName)
	if err != nil {
		return fmt.Errorf("service %q is not installed", serviceName)
	}
	defer s.Close()

	if err := s.Delete(); err != nil {
		return fmt.Errorf("deleting service: %w", err)
	}

	// Remove event log source
	_ = eventlog.Remove(serviceName)

	return nil
}
