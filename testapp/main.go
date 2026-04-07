// testapp is a controllable dummy service for testing ELNSSM.
// It provides a web UI with buttons to simulate crashes, errors,
// health state changes, and other failure scenarios.
package main

import (
	"embed"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
)

//go:embed index.html
var staticFiles embed.FS

// State holds all toggleable application state.
type State struct {
	mu              sync.RWMutex
	healthy         bool
	apiEnabled      bool
	stderrSpamStop  chan struct{}
	stdoutSpamStop  chan struct{}
	stderrSpamming  bool
	stdoutSpamming  bool
	memBallast      [][]byte
	slowShutdownSec int
	startTime       time.Time
}

var state = &State{
	healthy:         true,
	apiEnabled:      true,
	slowShutdownSec: 0,
	startTime:       time.Now(),
}

func main() {
	port := "8550"
	if p := os.Getenv("PORT"); p != "" {
		port = p
	}

	mux := http.NewServeMux()

	// --- Static UI ---
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		data, _ := staticFiles.ReadFile("index.html")
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write(data)
	})

	// --- Health endpoint (for ELNSSM health checks) ---
	mux.HandleFunc("/health", handleHealth)

	// --- API check endpoint (for curl checks) ---
	mux.HandleFunc("/api/check", handleAPICheck)

	// --- Status (returns current state as JSON) ---
	mux.HandleFunc("/api/status", handleStatus)

	// --- Action endpoints ---
	mux.HandleFunc("/action/crash", handleCrash)
	mux.HandleFunc("/action/panic", handlePanic)
	mux.HandleFunc("/action/exit", handleExitCode)
	mux.HandleFunc("/action/hang", handleHang)
	mux.HandleFunc("/action/toggle-health", handleToggleHealth)
	mux.HandleFunc("/action/toggle-api", handleToggleAPI)
	mux.HandleFunc("/action/stderr-spam", handleStderrSpam)
	mux.HandleFunc("/action/stdout-spam", handleStdoutSpam)
	mux.HandleFunc("/action/memory-leak", handleMemoryLeak)
	mux.HandleFunc("/action/free-memory", handleFreeMemory)
	mux.HandleFunc("/action/cpu-hog", handleCPUHog)
	mux.HandleFunc("/action/slow-shutdown", handleSlowShutdown)

	// Graceful shutdown with configurable delay
	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		sig := <-sigCh

		state.mu.RLock()
		delay := state.slowShutdownSec
		state.mu.RUnlock()

		if delay > 0 {
			fmt.Fprintf(os.Stderr, "[testapp] received %v, slow shutdown in %d seconds...\n", sig, delay)
			time.Sleep(time.Duration(delay) * time.Second)
		}
		fmt.Fprintf(os.Stderr, "[testapp] shutting down\n")
		os.Exit(0)
	}()

	fmt.Printf("[testapp] starting on http://127.0.0.1:%s\n", port)
	fmt.Printf("[testapp] health: http://127.0.0.1:%s/health\n", port)
	fmt.Printf("[testapp] api:    http://127.0.0.1:%s/api/check\n", port)
	log.Fatal(http.ListenAndServe("0.0.0.0:"+port, mux))
}

// ---------- Handlers ----------

func handleHealth(w http.ResponseWriter, r *http.Request) {
	state.mu.RLock()
	h := state.healthy
	state.mu.RUnlock()

	if h {
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `{"status":"healthy"}`)
	} else {
		w.WriteHeader(http.StatusServiceUnavailable)
		fmt.Fprint(w, `{"status":"unhealthy"}`)
	}
}

func handleAPICheck(w http.ResponseWriter, r *http.Request) {
	state.mu.RLock()
	enabled := state.apiEnabled
	state.mu.RUnlock()

	if enabled {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, `{"status":"ok","uptime":"%s"}`, time.Since(state.startTime).Round(time.Second))
	} else {
		w.WriteHeader(http.StatusServiceUnavailable)
		fmt.Fprint(w, `{"status":"disabled"}`)
	}
}

func handleStatus(w http.ResponseWriter, r *http.Request) {
	state.mu.RLock()
	defer state.mu.RUnlock()

	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	resp := map[string]any{
		"healthy":          state.healthy,
		"api_enabled":      state.apiEnabled,
		"stderr_spamming":  state.stderrSpamming,
		"stdout_spamming":  state.stdoutSpamming,
		"memory_ballast":   len(state.memBallast),
		"alloc_mb":         memStats.Alloc / 1024 / 1024,
		"slow_shutdown_sec": state.slowShutdownSec,
		"uptime":           time.Since(state.startTime).Round(time.Second).String(),
		"pid":              os.Getpid(),
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func handleCrash(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintln(os.Stderr, "[testapp] CRASH triggered via UI - calling os.Exit(1)")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("crashing..."))
	go func() {
		time.Sleep(100 * time.Millisecond)
		os.Exit(1)
	}()
}

func handlePanic(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintln(os.Stderr, "[testapp] PANIC triggered via UI")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("panicking..."))
	go func() {
		time.Sleep(100 * time.Millisecond)
		panic("testapp: intentional panic triggered via UI")
	}()
}

func handleExitCode(w http.ResponseWriter, r *http.Request) {
	code := 0
	if c := r.URL.Query().Get("code"); c != "" {
		fmt.Sscanf(c, "%d", &code)
	}
	fmt.Fprintf(os.Stderr, "[testapp] EXIT with code %d triggered via UI\n", code)
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, "exiting with code %d...", code)
	go func() {
		time.Sleep(100 * time.Millisecond)
		os.Exit(code)
	}()
}

func handleHang(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintln(os.Stderr, "[testapp] HANG triggered via UI - blocking all goroutines")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("hanging..."))
	go func() {
		time.Sleep(100 * time.Millisecond)
		// Block forever with a channel that nobody writes to
		select {}
	}()
	// Also block all new HTTP handlers
	go func() {
		time.Sleep(200 * time.Millisecond)
		runtime.Goexit()
	}()
}

func handleToggleHealth(w http.ResponseWriter, r *http.Request) {
	state.mu.Lock()
	state.healthy = !state.healthy
	h := state.healthy
	state.mu.Unlock()

	fmt.Fprintf(os.Stderr, "[testapp] health toggled -> %v\n", h)
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, `{"healthy":%v}`, h)
}

func handleToggleAPI(w http.ResponseWriter, r *http.Request) {
	state.mu.Lock()
	state.apiEnabled = !state.apiEnabled
	e := state.apiEnabled
	state.mu.Unlock()

	fmt.Fprintf(os.Stderr, "[testapp] api toggled -> %v\n", e)
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, `{"api_enabled":%v}`, e)
}

func handleStderrSpam(w http.ResponseWriter, r *http.Request) {
	state.mu.Lock()
	defer state.mu.Unlock()

	if state.stderrSpamming {
		close(state.stderrSpamStop)
		state.stderrSpamming = false
		fmt.Fprintln(os.Stderr, "[testapp] stderr spam stopped")
		fmt.Fprint(w, `{"stderr_spamming":false}`)
		return
	}

	state.stderrSpamStop = make(chan struct{})
	state.stderrSpamming = true
	stop := state.stderrSpamStop
	go func() {
		i := 0
		for {
			select {
			case <-stop:
				return
			default:
				fmt.Fprintf(os.Stderr, "[testapp] ERROR #%d: simulated error message at %s\n", i, time.Now().Format(time.RFC3339))
				i++
				time.Sleep(500 * time.Millisecond)
			}
		}
	}()
	fmt.Fprint(w, `{"stderr_spamming":true}`)
}

func handleStdoutSpam(w http.ResponseWriter, r *http.Request) {
	state.mu.Lock()
	defer state.mu.Unlock()

	if state.stdoutSpamming {
		close(state.stdoutSpamStop)
		state.stdoutSpamming = false
		fmt.Fprintln(os.Stdout, "[testapp] stdout spam stopped")
		fmt.Fprint(w, `{"stdout_spamming":false}`)
		return
	}

	state.stdoutSpamStop = make(chan struct{})
	state.stdoutSpamming = true
	stop := state.stdoutSpamStop
	go func() {
		i := 0
		for {
			select {
			case <-stop:
				return
			default:
				fmt.Fprintf(os.Stdout, "[testapp] INFO #%d: simulated log output at %s\n", i, time.Now().Format(time.RFC3339))
				i++
				time.Sleep(200 * time.Millisecond)
			}
		}
	}()
	fmt.Fprint(w, `{"stdout_spamming":true}`)
}

var memLeakCounter atomic.Int64

func handleMemoryLeak(w http.ResponseWriter, r *http.Request) {
	state.mu.Lock()
	// Allocate 50MB chunk
	chunk := make([]byte, 50*1024*1024)
	for i := range chunk {
		chunk[i] = byte(i % 256)
	}
	state.memBallast = append(state.memBallast, chunk)
	count := len(state.memBallast)
	state.mu.Unlock()

	totalMB := count * 50
	fmt.Fprintf(os.Stderr, "[testapp] memory leak: allocated chunk #%d (total ballast: %d MB)\n", count, totalMB)
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, `{"chunks":%d,"total_mb":%d}`, count, totalMB)
}

func handleFreeMemory(w http.ResponseWriter, r *http.Request) {
	state.mu.Lock()
	state.memBallast = nil
	state.mu.Unlock()
	runtime.GC()

	fmt.Fprintln(os.Stderr, "[testapp] memory freed")
	fmt.Fprint(w, `{"chunks":0,"total_mb":0}`)
}

func handleCPUHog(w http.ResponseWriter, r *http.Request) {
	secs := 10
	if s := r.URL.Query().Get("seconds"); s != "" {
		fmt.Sscanf(s, "%d", &secs)
	}
	if secs > 120 {
		secs = 120
	}

	cores := runtime.NumCPU()
	fmt.Fprintf(os.Stderr, "[testapp] CPU hog: burning %d cores for %d seconds\n", cores, secs)

	for i := 0; i < cores; i++ {
		go func() {
			deadline := time.Now().Add(time.Duration(secs) * time.Second)
			for time.Now().Before(deadline) {
				// burn CPU
				_ = 1 + 1
			}
		}()
	}

	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, `{"cpu_hog":true,"cores":%d,"seconds":%d}`, cores, secs)
}

func handleSlowShutdown(w http.ResponseWriter, r *http.Request) {
	secs := 30
	if s := r.URL.Query().Get("seconds"); s != "" {
		fmt.Sscanf(s, "%d", &secs)
	}

	state.mu.Lock()
	state.slowShutdownSec = secs
	state.mu.Unlock()

	fmt.Fprintf(os.Stderr, "[testapp] slow shutdown set to %d seconds\n", secs)
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, `{"slow_shutdown_sec":%d}`, secs)
}
