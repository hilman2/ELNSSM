package process

import (
	"context"
	"sync"
	"time"
	"unsafe"
)

// memoryStatusEx maps to the Win32 MEMORYSTATUSEX structure.
type memoryStatusEx struct {
	Length               uint32
	MemoryLoad           uint32
	TotalPhys            uint64
	AvailPhys            uint64
	TotalPageFile        uint64
	AvailPageFile        uint64
	TotalVirtual         uint64
	AvailVirtual         uint64
	AvailExtendedVirtual uint64
}

var (
	procGetSystemTimes       = modKernel32.NewProc("GetSystemTimes")
	procGlobalMemoryStatusEx = modKernel32.NewProc("GlobalMemoryStatusEx")
)

// HostSample holds a single host-level resource measurement.
type HostSample struct {
	CPUPercent    float64 `json:"cpu_percent"`
	MemoryTotal   int64   `json:"memory_total"`
	MemoryUsed    int64   `json:"memory_used"`
	MemoryPercent float64 `json:"memory_percent"`
}

// HostMonitor periodically samples host-level CPU and RAM.
type HostMonitor struct {
	interval time.Duration
	latest   HostSample
	mu       sync.RWMutex

	// CPU tracking between samples
	prevIdle  int64
	prevTotal int64
}

// NewHostMonitor creates a new host monitor with the given sampling interval.
func NewHostMonitor(interval time.Duration) *HostMonitor {
	if interval <= 0 {
		interval = 2 * time.Second
	}
	return &HostMonitor{
		interval: interval,
	}
}

// Run starts the monitoring loop. Blocks until ctx is cancelled.
func (hm *HostMonitor) Run(ctx context.Context) {
	// Take an initial sample to seed CPU delta tracking
	hm.takeSample()

	ticker := time.NewTicker(hm.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			hm.takeSample()
		}
	}
}

// Latest returns the most recent host resource sample.
func (hm *HostMonitor) Latest() HostSample {
	hm.mu.RLock()
	defer hm.mu.RUnlock()
	return hm.latest
}

func (hm *HostMonitor) takeSample() {
	sample := HostSample{}

	// CPU via GetSystemTimes
	var idleTime, kernelTime, userTime [8]byte // FILETIME = 8 bytes
	ret, _, _ := procGetSystemTimes.Call(
		uintptr(unsafe.Pointer(&idleTime)),   //nolint:gosec // Win32 API binding
		uintptr(unsafe.Pointer(&kernelTime)), //nolint:gosec // Win32 API binding
		uintptr(unsafe.Pointer(&userTime)),   //nolint:gosec // Win32 API binding
	)
	if ret != 0 {
		idle := filetimeToInt64FromBytes(idleTime)
		kernel := filetimeToInt64FromBytes(kernelTime)
		user := filetimeToInt64FromBytes(userTime)

		// kernelTime includes idleTime, so total = kernel + user
		total := kernel + user

		if hm.prevTotal > 0 {
			deltaTotal := total - hm.prevTotal
			deltaIdle := idle - hm.prevIdle
			if deltaTotal > 0 {
				sample.CPUPercent = float64(deltaTotal-deltaIdle) / float64(deltaTotal) * 100.0
			}
		}

		hm.prevIdle = idle
		hm.prevTotal = total
	}

	// RAM via GlobalMemoryStatusEx
	var memStatus memoryStatusEx
	memStatus.Length = uint32(unsafe.Sizeof(memStatus))
	ret, _, _ = procGlobalMemoryStatusEx.Call(uintptr(unsafe.Pointer(&memStatus))) //nolint:gosec // Win32 API binding
	if ret != 0 {
		// Physical RAM in bytes fits comfortably in int64 (~9.2 EB max).
		sample.MemoryTotal = int64(memStatus.TotalPhys)                      //nolint:gosec // see comment above
		sample.MemoryUsed = int64(memStatus.TotalPhys - memStatus.AvailPhys) //nolint:gosec // see comment above
		sample.MemoryPercent = float64(memStatus.MemoryLoad)
	}

	hm.mu.Lock()
	hm.latest = sample
	hm.mu.Unlock()
}

func filetimeToInt64FromBytes(ft [8]byte) int64 {
	lo := uint32(ft[0]) | uint32(ft[1])<<8 | uint32(ft[2])<<16 | uint32(ft[3])<<24
	hi := uint32(ft[4]) | uint32(ft[5])<<8 | uint32(ft[6])<<16 | uint32(ft[7])<<24
	return int64(hi)<<32 | int64(lo)
}
