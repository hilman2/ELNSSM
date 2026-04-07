package api

import (
	"net/http"
	"sync"
	"time"
)

// RateLimiter is an in-memory token-bucket rate limiter per IP.
type RateLimiter struct {
	mu      sync.Mutex
	clients map[string]*bucket
	rate    int
	window  time.Duration
}

type bucket struct {
	tokens    int
	lastReset time.Time
}

// NewRateLimiter creates a rate limiter allowing 'rate' requests per 'window' per IP.
func NewRateLimiter(rate int, window time.Duration) *RateLimiter {
	rl := &RateLimiter{
		clients: make(map[string]*bucket),
		rate:    rate,
		window:  window,
	}
	go rl.cleanup()
	return rl
}

// Allow returns true if the request from the given IP is within the rate limit.
func (rl *RateLimiter) Allow(ip string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	b, ok := rl.clients[ip]
	if !ok || now.Sub(b.lastReset) >= rl.window {
		rl.clients[ip] = &bucket{tokens: rl.rate - 1, lastReset: now}
		return true
	}

	if b.tokens > 0 {
		b.tokens--
		return true
	}
	return false
}

func (rl *RateLimiter) cleanup() {
	ticker := time.NewTicker(rl.window)
	defer ticker.Stop()
	for range ticker.C {
		rl.mu.Lock()
		now := time.Now()
		for ip, b := range rl.clients {
			if now.Sub(b.lastReset) >= rl.window*2 {
				delete(rl.clients, ip)
			}
		}
		rl.mu.Unlock()
	}
}

// rateLimitMiddleware returns chi-compatible middleware that enforces rate limiting.
func rateLimitMiddleware(rl *RateLimiter) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip := extractIP(r)
			if !rl.Allow(ip) {
				writeError(w, http.StatusTooManyRequests, "RATE_LIMITED", "Too many requests")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
