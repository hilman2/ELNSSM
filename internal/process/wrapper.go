package process

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os/exec"
	"strings"
	"sync"
	"syscall"
	"time"
	"unsafe"

	"github.com/hilman2/ELNSSM/internal/model"
	"golang.org/x/sys/windows"
)

// ExitResult holds the outcome of a process exit.
type ExitResult struct {
	ExitCode int
	Error    error
	Crashed  bool
}

// Wrapper manages a single child process with full lifecycle control.
type Wrapper struct {
	config    *model.Service
	cmd       *exec.Cmd
	jobObject *JobObject
	pid       int
	exitCh    chan ExitResult
	stdout    io.ReadCloser
	stderr    io.ReadCloser
	cancel    context.CancelFunc
	mu        sync.Mutex
	running   bool
}

// NewWrapper creates a new process wrapper for the given service configuration.
func NewWrapper(svc *model.Service) *Wrapper {
	return &Wrapper{
		config: svc,
		exitCh: make(chan ExitResult, 1),
	}
}

// Start spawns the child process, assigns it to a Job Object, and begins I/O capture.
func (w *Wrapper) Start(ctx context.Context) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.running {
		return fmt.Errorf("process is already running")
	}

	// Create Job Object
	job, err := NewJobObject()
	if err != nil {
		return fmt.Errorf("creating job object: %w", err)
	}
	w.jobObject = job

	// Build command
	innerCtx, cancel := context.WithCancel(ctx)
	w.cancel = cancel

	cmd := exec.CommandContext(innerCtx, w.config.Executable, w.config.Arguments...)
	cmd.Dir = w.config.WorkingDir

	// Set environment
	if len(w.config.Environment) > 0 {
		cmd.Env = buildEnvBlock(w.config.Environment)
	}

	// Create new process group so we can send ctrl signals to just this group
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP,
	}

	// Run-as-user: if a service account with encrypted password is configured
	if w.config.ServiceAccount != "" && w.config.ServiceAccountPassword != "" {
		token, tokenErr := logonServiceUser(w.config.ServiceAccount, w.config.ServiceAccountPassword)
		if tokenErr != nil {
			cancel()
			return fmt.Errorf("logon as %q: %w", w.config.ServiceAccount, tokenErr)
		}
		cmd.SysProcAttr.Token = token
	}

	// Set up pipes
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		job.Close()
		cancel()
		return fmt.Errorf("creating stdout pipe: %w", err)
	}
	w.stdout = stdout

	stderr, err := cmd.StderrPipe()
	if err != nil {
		job.Close()
		cancel()
		return fmt.Errorf("creating stderr pipe: %w", err)
	}
	w.stderr = stderr

	// Start process
	if err := cmd.Start(); err != nil {
		job.Close()
		cancel()
		return fmt.Errorf("starting process: %w", err)
	}

	w.cmd = cmd
	w.pid = cmd.Process.Pid

	// Assign to Job Object
	processHandle, err := windows.OpenProcess(
		windows.PROCESS_SET_QUOTA|windows.PROCESS_TERMINATE,
		false,
		uint32(w.pid),
	)
	if err != nil {
		slog.Warn("Could not open process for job assignment", "pid", w.pid, "error", err)
	} else {
		if err := job.Assign(processHandle); err != nil {
			slog.Warn("Could not assign process to job object", "pid", w.pid, "error", err)
		}
		windows.CloseHandle(processHandle)
	}

	// Set process priority
	if w.config.Priority != "" && w.config.Priority != model.PriorityNormal {
		setPriority(uint32(w.pid), w.config.Priority)
	}

	w.running = true

	// Wait goroutine
	go func() {
		err := cmd.Wait()
		exitCode := 0
		crashed := false

		if err != nil {
			if exitErr, ok := err.(*exec.ExitError); ok {
				exitCode = exitErr.ExitCode()
				crashed = true
			} else if innerCtx.Err() != nil {
				// Context cancelled (intentional stop)
				crashed = false
			} else {
				crashed = true
			}
		}

		w.mu.Lock()
		w.running = false
		w.mu.Unlock()

		w.exitCh <- ExitResult{
			ExitCode: exitCode,
			Error:    err,
			Crashed:  crashed,
		}
	}()

	slog.Info("Process started", "service", w.config.ID, "pid", w.pid)
	return nil
}

// Stop gracefully stops the child process.
func (w *Wrapper) Stop(ctx context.Context) error {
	w.mu.Lock()
	if !w.running {
		w.mu.Unlock()
		return nil
	}
	pid := uint32(w.pid)
	w.mu.Unlock()

	timeout := w.config.StopTimeout
	if timeout == 0 {
		timeout = 30 * time.Second
	}

	slog.Info("Stopping process", "service", w.config.ID, "pid", pid, "signal", w.config.StopSignal)

	// Phase 1: Send graceful signal
	switch w.config.StopSignal {
	case model.StopSignalCtrlC, "":
		if err := SendCtrlC(pid); err != nil {
			slog.Debug("Failed to send CTRL_C", "error", err)
		}
	case model.StopSignalCtrlBreak:
		if err := SendCtrlBreak(pid); err != nil {
			slog.Debug("Failed to send CTRL_BREAK", "error", err)
		}
	case model.StopSignalWMClose:
		if err := SendWMClose(pid); err != nil {
			slog.Debug("Failed to send WM_CLOSE", "error", err)
		}
	case model.StopSignalTerminate:
		// Skip graceful, go straight to kill
		return w.forceKill()
	}

	// Phase 2: Wait for graceful exit
	select {
	case <-w.exitCh:
		slog.Info("Process stopped gracefully", "service", w.config.ID)
		return nil
	case <-time.After(timeout):
		slog.Warn("Graceful stop timed out, force killing", "service", w.config.ID, "timeout", timeout)
	case <-ctx.Done():
		slog.Warn("Stop context cancelled, force killing", "service", w.config.ID)
	}

	// Phase 3: Force kill via Job Object
	return w.forceKill()
}

func (w *Wrapper) forceKill() error {
	if w.jobObject != nil {
		if err := w.jobObject.Terminate(1); err != nil {
			slog.Warn("Job object terminate failed, killing process directly", "error", err)
			if w.cmd != nil && w.cmd.Process != nil {
				return w.cmd.Process.Kill()
			}
		}
	} else if w.cmd != nil && w.cmd.Process != nil {
		return w.cmd.Process.Kill()
	}
	return nil
}

// Wait returns a channel that receives the exit result when the process stops.
func (w *Wrapper) Wait() <-chan ExitResult {
	return w.exitCh
}

// PID returns the current process ID.
func (w *Wrapper) PID() int {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.pid
}

// IsRunning returns whether the process is currently running.
func (w *Wrapper) IsRunning() bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.running
}

// Stdout returns the stdout reader for log capture.
func (w *Wrapper) Stdout() io.ReadCloser {
	return w.stdout
}

// Stderr returns the stderr reader for log capture.
func (w *Wrapper) Stderr() io.ReadCloser {
	return w.stderr
}

// Close releases all resources and kills the process tree via Job Object.
func (w *Wrapper) Close() {
	if w.cancel != nil {
		w.cancel()
	}
	if w.jobObject != nil {
		// Ensure processes are killed when handle closes during normal shutdown
		w.jobObject.SetKillOnClose(true)
		w.jobObject.Close()
	}
}

// Detach releases the wrapper without killing the child process.
// The process continues running as an orphan. Used during Guardian restart.
func (w *Wrapper) Detach() {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.cancel != nil {
		w.cancel()
		w.cancel = nil
	}

	// Close pipes (process keeps running, output goes to /dev/null)
	if w.stdout != nil {
		w.stdout.Close()
		w.stdout = nil
	}
	if w.stderr != nil {
		w.stderr.Close()
		w.stderr = nil
	}

	// Release job object WITHOUT kill-on-close (process survives)
	if w.jobObject != nil {
		w.jobObject.Close()
		w.jobObject = nil
	}

	w.running = false
}

// Adopt connects to an already-running process by PID.
// Used after Guardian restart to re-adopt orphaned children.
func (w *Wrapper) Adopt(pid int) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.running {
		return fmt.Errorf("wrapper already has a running process")
	}

	// Verify the process is still alive
	handle, err := windows.OpenProcess(
		windows.PROCESS_QUERY_INFORMATION|windows.PROCESS_SET_QUOTA|windows.PROCESS_TERMINATE|windows.SYNCHRONIZE,
		false,
		uint32(pid),
	)
	if err != nil {
		return fmt.Errorf("process %d not found: %w", pid, err)
	}

	// Check if the process has already exited
	ret, _ := windows.WaitForSingleObject(handle, 0)
	if ret != uint32(windows.WAIT_TIMEOUT) {
		windows.CloseHandle(handle)
		return fmt.Errorf("process %d has already exited", pid)
	}

	// Create new Job Object and assign process
	job, err := NewJobObject()
	if err != nil {
		windows.CloseHandle(handle)
		return fmt.Errorf("creating job object: %w", err)
	}

	if err := job.Assign(handle); err != nil {
		slog.Warn("Could not assign adopted process to job object", "pid", pid, "error", err)
	}

	w.jobObject = job
	w.pid = pid
	w.running = true
	w.exitCh = make(chan ExitResult, 1)

	// Monitor the process for exit using the handle
	go func() {
		windows.WaitForSingleObject(handle, windows.INFINITE)

		var exitCode uint32
		windows.GetExitCodeProcess(handle, &exitCode)
		windows.CloseHandle(handle)

		w.mu.Lock()
		w.running = false
		w.mu.Unlock()

		crashed := exitCode != 0
		w.exitCh <- ExitResult{
			ExitCode: int(exitCode),
			Crashed:  crashed,
		}
	}()

	slog.Info("Adopted running process", "service", w.config.ID, "pid", pid)
	return nil
}

// buildEnvBlock creates an environment variable block from the service config,
// inheriting the parent process environment and overlaying service-specific vars.
func buildEnvBlock(svcEnv map[string]string) []string {
	// Start from parent environment
	env := syscall.Environ()
	envMap := make(map[string]string)
	for _, e := range env {
		for i := 0; i < len(e); i++ {
			if e[i] == '=' {
				envMap[e[:i]] = e[i+1:]
				break
			}
		}
	}

	// Overlay service-specific vars
	for k, v := range svcEnv {
		envMap[k] = v
	}

	// Convert back to slice
	result := make([]string, 0, len(envMap))
	for k, v := range envMap {
		result = append(result, k+"="+v)
	}
	return result
}

var (
	modAdvapi32     = windows.NewLazySystemDLL("advapi32.dll")
	procLogonUserW  = modAdvapi32.NewProc("LogonUserW")
)

// logonServiceUser performs Windows LogonUser for run-as-user support.
// It accepts the DPAPI-encrypted password and decrypts it before logon.
func logonServiceUser(account, encryptedPassword string) (syscall.Token, error) {
	password, err := DecryptPassword(encryptedPassword)
	if err != nil {
		return 0, fmt.Errorf("decrypting password: %w", err)
	}

	// Parse domain\user or just user
	user := account
	domain := "."
	if idx := strings.IndexAny(account, `\/`); idx >= 0 {
		domain = account[:idx]
		user = account[idx+1:]
	}

	const logon32LogonService uint32 = 5
	const logon32ProviderDefault uint32 = 0

	var token syscall.Token
	userPtr, _ := syscall.UTF16PtrFromString(user)
	domainPtr, _ := syscall.UTF16PtrFromString(domain)
	passwordPtr, _ := syscall.UTF16PtrFromString(password)

	ret, _, callErr := procLogonUserW.Call(
		uintptr(unsafe.Pointer(userPtr)),
		uintptr(unsafe.Pointer(domainPtr)),
		uintptr(unsafe.Pointer(passwordPtr)),
		uintptr(logon32LogonService),
		uintptr(logon32ProviderDefault),
		uintptr(unsafe.Pointer(&token)),
	)
	if ret == 0 {
		return 0, fmt.Errorf("LogonUser: %w", callErr)
	}

	return token, nil
}

// setPriority sets the process priority class.
func setPriority(pid uint32, priority model.ProcessPriority) {
	handle, err := windows.OpenProcess(windows.PROCESS_SET_INFORMATION, false, pid)
	if err != nil {
		slog.Debug("Could not open process for priority", "error", err)
		return
	}
	defer windows.CloseHandle(handle)

	var class uint32
	switch priority {
	case model.PriorityIdle:
		class = windows.IDLE_PRIORITY_CLASS
	case model.PriorityBelowNormal:
		class = windows.BELOW_NORMAL_PRIORITY_CLASS
	case model.PriorityAboveNormal:
		class = windows.ABOVE_NORMAL_PRIORITY_CLASS
	case model.PriorityHigh:
		class = windows.HIGH_PRIORITY_CLASS
	case model.PriorityRealtime:
		class = windows.REALTIME_PRIORITY_CLASS
	default:
		return
	}

	if err := windows.SetPriorityClass(handle, class); err != nil {
		slog.Debug("Could not set process priority", "error", err)
	}
}
