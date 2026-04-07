package process

import (
	"fmt"
	"unsafe"

	"golang.org/x/sys/windows"
)

// JobObject wraps a Windows Job Object for process tree management.
type JobObject struct {
	handle windows.Handle
}

// NewJobObject creates a new Windows Job Object.
// Processes are manually terminated via Terminate() during normal stop.
// The job does NOT auto-kill on handle close, allowing graceful Guardian restarts.
func NewJobObject() (*JobObject, error) {
	handle, err := windows.CreateJobObject(nil, nil)
	if err != nil {
		return nil, fmt.Errorf("creating job object: %w", err)
	}

	return &JobObject{handle: handle}, nil
}

// Assign adds a process to the job object.
func (j *JobObject) Assign(process windows.Handle) error {
	if err := windows.AssignProcessToJobObject(j.handle, process); err != nil {
		return fmt.Errorf("assigning process to job object: %w", err)
	}
	return nil
}

// Terminate kills all processes in the job object.
func (j *JobObject) Terminate(exitCode uint32) error {
	if err := windows.TerminateJobObject(j.handle, exitCode); err != nil {
		return fmt.Errorf("terminating job object: %w", err)
	}
	return nil
}

// SetKillOnClose configures the job to kill all processes when the handle is closed.
// Used during normal shutdown to ensure cleanup. NOT set during restart.
func (j *JobObject) SetKillOnClose(kill bool) error {
	var flags uint32
	if kill {
		flags = windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE
	}
	info := windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION{
		BasicLimitInformation: windows.JOBOBJECT_BASIC_LIMIT_INFORMATION{
			LimitFlags: flags,
		},
	}
	_, err := windows.SetInformationJobObject(
		j.handle,
		windows.JobObjectExtendedLimitInformation,
		uintptr(unsafe.Pointer(&info)), //nolint:gosec // Win32 API binding
		uint32(unsafe.Sizeof(info)),
	)
	if err != nil {
		return fmt.Errorf("setting kill-on-close: %w", err)
	}
	return nil
}

// Close releases the job object handle.
func (j *JobObject) Close() error {
	if j.handle != 0 {
		err := windows.CloseHandle(j.handle)
		j.handle = 0
		return err
	}
	return nil
}
