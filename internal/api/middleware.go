package api

import (
	"log/slog"
	"net"
	"net/http"
	"time"

	"github.com/hilman2/ELNSSM/internal/config"
)

// ipWhitelist returns middleware that only allows requests from whitelisted IPs.
// An empty whitelist denies all requests.
func ipWhitelist(cfg *config.APIConfig) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if len(cfg.IPWhitelist) == 0 {
				slog.Warn("IP whitelist is empty, denying all requests")
				writeError(w, http.StatusForbidden, "FORBIDDEN", "IP address not allowed")
				return
			}

			clientIP := extractIP(r)
			for _, allowed := range cfg.IPWhitelist {
				if clientIP == allowed {
					next.ServeHTTP(w, r)
					return
				}
			}

			slog.Warn("Request blocked by IP whitelist", "ip", clientIP)
			writeError(w, http.StatusForbidden, "FORBIDDEN", "IP address not allowed")
		})
	}
}

// requestLogger logs incoming HTTP requests.
// Write operations (POST/PUT/DELETE) are logged at Info level, reads at Debug.
func requestLogger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		wrapped := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}
		next.ServeHTTP(wrapped, r)

		attrs := []any{
			"method", r.Method,
			"path", r.URL.Path,
			"status", wrapped.statusCode,
			"duration", time.Since(start),
			"ip", extractIP(r),
		}

		// Include auth identity for write operations (audit trail)
		switch r.Method {
		case http.MethodPost, http.MethodPut, http.MethodDelete:
			attrs = append(attrs, "user", AuthUser(r))
			slog.Info("HTTP request", attrs...)
		default:
			slog.Debug("HTTP request", attrs...)
		}
	})
}

// securityHeaders adds security-related HTTP headers to all responses.
func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Content-Security-Policy",
			"default-src 'self'; script-src 'self' 'unsafe-inline'; "+
				"style-src 'self' 'unsafe-inline'; font-src 'self'; "+
				"connect-src 'self' ws: wss:; img-src 'self' data:; frame-ancestors 'none'")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		w.Header().Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
		next.ServeHTTP(w, r)
	})
}

// extractIP returns the client IP from r.RemoteAddr only.
// Proxy headers (X-Forwarded-For, X-Real-IP) are not trusted.
// IPv4-mapped IPv6 addresses (::ffff:127.0.0.1) are normalized to IPv4.
func extractIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	// Normalize IPv4-mapped IPv6 (e.g. ::ffff:127.0.0.1 → 127.0.0.1)
	// so whitelist matching works consistently.
	ip := net.ParseIP(host)
	if ip == nil {
		return host
	}
	if v4 := ip.To4(); v4 != nil {
		return v4.String()
	}
	return ip.String()
}

type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (w *responseWriter) WriteHeader(code int) {
	w.statusCode = code
	w.ResponseWriter.WriteHeader(code)
}
