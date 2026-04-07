package process

import (
	"context"
	"log/slog"
	"sort"
	"sync"
	"time"
	"unsafe"

	"github.com/hilman2/ELNSSM/internal/model"
	"golang.org/x/sys/windows"
)

// PROCESS_MEMORY_COUNTERS_EX holds memory info from K32GetProcessMemoryInfo.
type processMemoryCountersEx struct {
	CB                         uint32
	PageFaultCount             uint32
	PeakWorkingSetSize         uintptr
	WorkingSetSize             uintptr
	QuotaPeakPagedPoolUsage    uintptr
	QuotaPagedPoolUsage        uintptr
	QuotaPeakNonPagedPoolUsage uintptr
	QuotaNonPagedPoolUsage     uintptr
	PagefileUsage              uintptr
	PeakPagefileUsage          uintptr
	PrivateUsage               uintptr
}

var (
	modKernel32                = windows.NewLazySystemDLL("kernel32.dll")
	procK32GetProcessMemoryInfo = modKernel32.NewProc("K32GetProcessMemoryInfo")
)

// ResourceSample holds a single resource measurement.
type ResourceSample struct {
	Timestamp   time.Time
	CPUPercent  float64
	MemoryBytes int64
}

// ResourceBreach describes a threshold violation.
type ResourceBreach struct {
	Type    string // "cpu_sustained", "memory_max", "memory_spike"
	Message string
}

// ResourceMonitorConfig configures the resource monitor.
type ResourceMonitorConfig struct {
	PID            int
	Limits         model.ResourceLimits
	CheckInterval  time.Duration
	GracePeriod    time.Duration // startup grace period before evaluation
}

// ResourceMonitor periodically samples CPU% and RAM for a process.
type ResourceMonitor struct {
	cfg       ResourceMonitorConfig
	breachCh  chan ResourceBreach
	latest    ResourceSample
	mu        sync.RWMutex

	// CPU tracking
	prevKernelTime int64
	prevUserTime   int64
	prevWallTime   time.Time

	// Memory baseline for spike detection
	baselineSamples []int64
	baselineMemory  int64 // median of first N samples
	baselineReady   bool
}

// NewResourceMonitor creates a new resource monitor.
func NewResourceMonitor(cfg ResourceMonitorConfig) *ResourceMonitor {
	if cfg.CheckInterval <= 0 {
		cfg.CheckInterval = 5 * time.Second
	}
	if cfg.GracePeriod <= 0 {
		cfg.GracePeriod = 30 * time.Second
	}
	return &ResourceMonitor{
		cfg:      cfg,
		breachCh: make(chan ResourceBreach, 8),
	}
}

// Run starts the monitoring loop. Blocks until ctx is cancelled.
func (rm *ResourceMonitor) Run(ctx context.Context) {
	ticker := time.NewTicker(rm.cfg.CheckInterval)
	defer ticker.Stop()

	startTime := time.Now()
	var cpuSamples []cpuWindowSample

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			sample := rm.takeSample()
			if sample == nil {
				continue
			}

			rm.mu.Lock()
			rm.latest = *sample
			rm.mu.Unlock()

			// Collect baseline samples for spike detection
			if !rm.baselineReady {
				rm.baselineSamples = append(rm.baselineSamples, sample.MemoryBytes)
				if len(rm.baselineSamples) >= 6 {
					rm.baselineMemory = medianInt64(rm.baselineSamples)
					rm.baselineReady = true
				}
			}

			// Skip evaluation during grace period
			if time.Since(startTime) < rm.cfg.GracePeriod {
				continue
			}

			// Evaluate thresholds
			if rm.cfg.Limits.CPUThreshold > 0 && rm.cfg.Limits.CPUDuration > 0 {
				cpuSamples = append(cpuSamples, cpuWindowSample{
					timestamp: sample.Timestamp,
					cpu:       sample.CPUPercent,
				})
				cpuSamples = pruneOldSamples(cpuSamples, rm.cfg.Limits.CPUDuration)
				if rm.evaluateCPUSustained(cpuSamples) {
					rm.sendBreach(ResourceBreach{
						Type:    "cpu_sustained",
						Message: "CPU sustained above threshold",
					})
				}
			}

			if rm.cfg.Limits.MemoryMax > 0 && sample.MemoryBytes > rm.cfg.Limits.MemoryMax {
				rm.sendBreach(ResourceBreach{
					Type:    "memory_max",
					Message: "Memory exceeds maximum limit",
				})
			}

			if rm.cfg.Limits.MemorySpikeRatio > 0 && rm.baselineReady && rm.baselineMemory > 0 {
				ratio := float64(sample.MemoryBytes) / float64(rm.baselineMemory)
				if ratio >= rm.cfg.Limits.MemorySpikeRatio {
					rm.sendBreach(ResourceBreach{
						Type:    "memory_spike",
						Message: "Memory spike detected",
					})
				}
			}
		}
	}
}

// BreachCh returns a channel that receives resource threshold breaches.
func (rm *ResourceMonitor) BreachCh() <-chan ResourceBreach {
	return rm.breachCh
}

// Latest returns the most recent resource sample.
func (rm *ResourceMonitor) Latest() ResourceSample {
	rm.mu.RLock()
	defer rm.mu.RUnlock()
	return rm.latest
}

func (rm *ResourceMonitor) sendBreach(b ResourceBreach) {
	select {
	case rm.breachCh <- b:
	default:
		slog.Warn("Resource breach channel full, dropping", "type", b.Type)
	}
}

func (rm *ResourceMonitor) takeSample() *ResourceSample {
	handle, err := windows.OpenProcess(
		windows.PROCESS_QUERY_LIMITED_INFORMATION,
		false,
		uint32(rm.cfg.PID),
	)
	if err != nil {
		return nil
	}
	defer windows.CloseHandle(handle)

	// CPU: GetProcessTimes
	var creation, exit, kernel, user windows.Filetime
	if err := windows.GetProcessTimes(handle, &creation, &exit, &kernel, &user); err != nil {
		return nil
	}

	kernelTime := filetimeToInt64(kernel)
	userTime := filetimeToInt64(user)
	now := time.Now()

	var cpuPercent float64
	if rm.prevWallTime.IsZero() {
		// First sample, can't compute CPU% yet
		rm.prevKernelTime = kernelTime
		rm.prevUserTime = userTime
		rm.prevWallTime = now
	} else {
		cpuDelta := float64((kernelTime - rm.prevKernelTime) + (userTime - rm.prevUserTime))
		wallDelta := float64(now.Sub(rm.prevWallTime).Nanoseconds() / 100) // FILETIME is in 100ns units
		if wallDelta > 0 {
			cpuPercent = (cpuDelta / wallDelta) * 100.0
		}
		rm.prevKernelTime = kernelTime
		rm.prevUserTime = userTime
		rm.prevWallTime = now
	}

	// Memory: K32GetProcessMemoryInfo
	var memCounters processMemoryCountersEx
	memCounters.CB = uint32(unsafe.Sizeof(memCounters))
	ret, _, _ := procK32GetProcessMemoryInfo.Call(
		uintptr(handle),
		uintptr(unsafe.Pointer(&memCounters)),
		uintptr(memCounters.CB),
	)
	var memoryBytes int64
	if ret != 0 {
		memoryBytes = int64(memCounters.WorkingSetSize)
	}

	return &ResourceSample{
		Timestamp:   now,
		CPUPercent:  cpuPercent,
		MemoryBytes: memoryBytes,
	}
}

type cpuWindowSample struct {
	timestamp time.Time
	cpu       float64
}

func pruneOldSamples(samples []cpuWindowSample, window time.Duration) []cpuWindowSample {
	cutoff := time.Now().Add(-window)
	i := 0
	for i < len(samples) && samples[i].timestamp.Before(cutoff) {
		i++
	}
	return samples[i:]
}

func (rm *ResourceMonitor) evaluateCPUSustained(samples []cpuWindowSample) bool {
	if len(samples) < 2 {
		return false
	}
	// Check if the window covers at least the configured duration
	window := samples[len(samples)-1].timestamp.Sub(samples[0].timestamp)
	if window < rm.cfg.Limits.CPUDuration {
		return false
	}
	// All samples must exceed the threshold
	for _, s := range samples {
		if s.cpu < rm.cfg.Limits.CPUThreshold {
			return false
		}
	}
	return true
}

func filetimeToInt64(ft windows.Filetime) int64 {
	return int64(ft.HighDateTime)<<32 | int64(ft.LowDateTime)
}

func medianInt64(values []int64) int64 {
	sorted := make([]int64, len(values))
	copy(sorted, values)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })
	n := len(sorted)
	if n%2 == 0 {
		return (sorted[n/2-1] + sorted[n/2]) / 2
	}
	return sorted[n/2]
}

