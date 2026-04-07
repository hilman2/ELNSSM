package process

import (
	"fmt"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	modkernel32                  = windows.NewLazySystemDLL("kernel32.dll")
	procGenerateConsoleCtrlEvent = modkernel32.NewProc("GenerateConsoleCtrlEvent")
)

// SendCtrlC sends a CTRL_C_EVENT to a process group.
func SendCtrlC(pid uint32) error {
	return sendCtrlEvent(0, pid) // CTRL_C_EVENT = 0
}

// SendCtrlBreak sends a CTRL_BREAK_EVENT to a process group.
func SendCtrlBreak(pid uint32) error {
	return sendCtrlEvent(1, pid) // CTRL_BREAK_EVENT = 1
}

func sendCtrlEvent(event, pid uint32) error {
	r1, _, err := procGenerateConsoleCtrlEvent.Call(
		uintptr(event),
		uintptr(pid),
	)
	if r1 == 0 {
		return fmt.Errorf("GenerateConsoleCtrlEvent(%d, %d): %w", event, pid, err)
	}
	return nil
}

// SendWMClose finds all top-level windows owned by the given PID and sends WM_CLOSE.
func SendWMClose(pid uint32) error {
	var lastErr error
	found := false

	moduser32 := windows.NewLazySystemDLL("user32.dll")
	procEnumWindows := moduser32.NewProc("EnumWindows")
	procGetWindowThreadProcessId := moduser32.NewProc("GetWindowThreadProcessId")
	procPostMessage := moduser32.NewProc("PostMessageW")

	const WM_CLOSE = 0x0010

	callback := windows.NewCallback(func(hwnd uintptr, lparam uintptr) uintptr {
		var windowPid uint32
		procGetWindowThreadProcessId.Call(hwnd, uintptr(unsafe.Pointer(&windowPid)))
		if windowPid == uint32(lparam) {
			found = true
			r1, _, err := procPostMessage.Call(hwnd, WM_CLOSE, 0, 0)
			if r1 == 0 {
				lastErr = fmt.Errorf("PostMessage WM_CLOSE: %w", err)
			}
		}
		return 1 // continue enumeration
	})

	r1, _, err := procEnumWindows.Call(callback, uintptr(pid))
	if r1 == 0 {
		return fmt.Errorf("EnumWindows: %w", err)
	}

	if !found {
		return fmt.Errorf("no windows found for PID %d", pid)
	}

	return lastErr
}
