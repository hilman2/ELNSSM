package api

import (
	"context"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"golang.org/x/crypto/bcrypt"

	"github.com/hilman2/ELNSSM/internal/config"
	"github.com/hilman2/ELNSSM/internal/sspi"
)

type contextKey string

const (
	ctxKeyAuthUser contextKey = "auth_user"
	ctxKeyClientIP contextKey = "client_ip"
	ctxKeyConnID   contextKey = "conn_id"
)

// AuthUser returns the authenticated username from the request context.
// Returns "anonymous" if auth is disabled, "token-user" for token auth.
func AuthUser(r *http.Request) string {
	if v, ok := r.Context().Value(ctxKeyAuthUser).(string); ok && v != "" {
		return v
	}
	return "anonymous"
}

// ClientIP returns the client IP from the request context.
func ClientIP(r *http.Request) string {
	if v, ok := r.Context().Value(ctxKeyClientIP).(string); ok && v != "" {
		return v
	}
	return extractIP(r)
}

func withAuthContext(r *http.Request, user, ip string) *http.Request {
	ctx := context.WithValue(r.Context(), ctxKeyAuthUser, user)
	ctx = context.WithValue(ctx, ctxKeyClientIP, ip)
	return r.WithContext(ctx)
}

// authMiddleware returns middleware that enforces authentication on API routes.
// Supports Bearer token auth, Basic auth, SSPI Negotiate auth, and WebSocket query param auth.
func authMiddleware(cfg *config.AuthConfig, sspiStore *sspi.SessionStore) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			clientIP := extractIP(r)

			if !cfg.Enabled {
				next.ServeHTTP(w, withAuthContext(r, "anonymous", clientIP))
				return
			}

			// Loopback callers can skip authentication, including the SSPI
			// admin check below. Whether that is acceptable depends on who
			// can reach the listener, so the reasoning lives with the option
			// in config.AuthConfig.AllowLocalBypass.
			if cfg.AllowLocalBypass && isLoopback(clientIP) {
				next.ServeHTTP(w, withAuthContext(r, "local-admin", clientIP))
				return
			}

			// SSPI Negotiate auth mode
			if cfg.Type == "sspi" && sspiStore != nil {
				connID := connIDFromRequest(r)

				// WebSocket upgrade: check session first, then Negotiate header, then ?token= fallback
				if isWebSocketUpgrade(r) {
					if sess := sspiStore.Get(connID); sess != nil && sess.State == sspi.StateComplete {
						next.ServeHTTP(w, withAuthContext(r, sess.Username, clientIP))
						return
					}
					auth := r.Header.Get("Authorization")
					if strings.HasPrefix(auth, "Negotiate ") {
						negotiateAuth(w, r, next, sspiStore, connID, clientIP)
						return
					}
					// Bearer fallback for WebSocket
					token := r.URL.Query().Get("token")
					if token != "" && cfg.TokenHash != "" {
						if bcrypt.CompareHashAndPassword([]byte(cfg.TokenHash), []byte(token)) == nil {
							next.ServeHTTP(w, withAuthContext(r, "token-user", clientIP))
							return
						}
					}
					slog.Warn("WebSocket SSPI auth failed", "ip", clientIP, "path", r.URL.Path)
					writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required")
					return
				}

				// Bearer fallback for CLI/scripts
				auth := r.Header.Get("Authorization")
				if strings.HasPrefix(auth, "Bearer ") {
					token := strings.TrimPrefix(auth, "Bearer ")
					if cfg.TokenHash != "" {
						if bcrypt.CompareHashAndPassword([]byte(cfg.TokenHash), []byte(token)) == nil {
							next.ServeHTTP(w, withAuthContext(r, "token-user", clientIP))
							return
						}
					}
					slog.Warn("Invalid bearer token", "ip", clientIP, "path", r.URL.Path)
					writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Invalid token")
					return
				}

				// Negotiate header present -> process handshake
				if strings.HasPrefix(auth, "Negotiate ") {
					negotiateAuth(w, r, next, sspiStore, connID, clientIP)
					return
				}

				// No auth header -> check if connection already authenticated
				if sess := sspiStore.Get(connID); sess != nil && sess.State == sspi.StateComplete {
					next.ServeHTTP(w, withAuthContext(r, sess.Username, clientIP))
					return
				}

				// Request Negotiate auth
				w.Header().Set("WWW-Authenticate", "Negotiate")
				writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required")
				return
			}

			// WebSocket upgrade requests: check ?token= query param
			if isWebSocketUpgrade(r) {
				token := r.URL.Query().Get("token")
				if token != "" && cfg.TokenHash != "" {
					if bcrypt.CompareHashAndPassword([]byte(cfg.TokenHash), []byte(token)) == nil {
						next.ServeHTTP(w, withAuthContext(r, "token-user", clientIP))
						return
					}
				}
				slog.Warn("WebSocket auth failed", "ip", clientIP, "path", r.URL.Path)
				writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required")
				return
			}

			auth := r.Header.Get("Authorization")
			if auth == "" {
				slog.Warn("Missing Authorization header", "ip", clientIP, "path", r.URL.Path)
				w.Header().Set("WWW-Authenticate", `Bearer realm="ELNSSM"`)
				writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required")
				return
			}

			// Bearer token auth
			if strings.HasPrefix(auth, "Bearer ") {
				token := strings.TrimPrefix(auth, "Bearer ")
				if cfg.TokenHash != "" {
					if bcrypt.CompareHashAndPassword([]byte(cfg.TokenHash), []byte(token)) == nil {
						next.ServeHTTP(w, withAuthContext(r, "token-user", clientIP))
						return
					}
				}
				slog.Warn("Invalid bearer token", "ip", clientIP, "path", r.URL.Path)
				writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Invalid token")
				return
			}

			// Basic auth
			if strings.HasPrefix(auth, "Basic ") {
				payload, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(auth, "Basic "))
				if err != nil {
					writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Invalid credentials")
					return
				}
				parts := strings.SplitN(string(payload), ":", 2)
				if len(parts) != 2 {
					writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Invalid credentials")
					return
				}
				username, password := parts[0], parts[1]

				if cfg.Username != "" && cfg.PasswordHash != "" {
					usernameMatch := subtle.ConstantTimeCompare([]byte(username), []byte(cfg.Username)) == 1
					passwordMatch := bcrypt.CompareHashAndPassword([]byte(cfg.PasswordHash), []byte(password)) == nil
					if usernameMatch && passwordMatch {
						next.ServeHTTP(w, withAuthContext(r, username, clientIP))
						return
					}
				}
				slog.Warn("Invalid basic auth credentials", "ip", clientIP, "path", r.URL.Path, "user", username)
				writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Invalid credentials")
				return
			}

			writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Unsupported authentication method")
		})
	}
}

// negotiateAuth handles a single step of the SSPI Negotiate handshake.
func negotiateAuth(w http.ResponseWriter, r *http.Request, next http.Handler, store *sspi.SessionStore, connID, clientIP string) {
	auth := r.Header.Get("Authorization")
	tokenB64 := strings.TrimPrefix(auth, "Negotiate ")
	clientToken, err := base64.StdEncoding.DecodeString(tokenB64)
	if err != nil {
		slog.Warn("Negotiate: invalid base64", "ip", clientIP, "error", err)
		store.Remove(connID)
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Invalid Negotiate token")
		return
	}

	sess := store.GetOrCreate(connID)

	// Determine input context (nil for first call, existing for continuation)
	var ctxIn *sspi.SecHandle
	if sess.State == sspi.StateChallenge && sess.CtxHandle != nil {
		ctxIn = sess.CtxHandle
	}

	outputToken, ctxOut, status, err := sspi.AcceptSecurityContext(store.Cred(), ctxIn, clientToken)
	if err != nil {
		slog.Warn("Negotiate: AcceptSecurityContext failed", "ip", clientIP, "error", err)
		store.Remove(connID)
		w.Header().Set("WWW-Authenticate", "Negotiate")
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Negotiate authentication failed")
		return
	}

	switch status {
	case sspi.StatusContinueNeeded:
		// Multi-leg: send challenge back, keep context for next round
		sess.CtxHandle = ctxOut
		sess.State = sspi.StateChallenge
		challenge := base64.StdEncoding.EncodeToString(outputToken)
		w.Header().Set("WWW-Authenticate", "Negotiate "+challenge)
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Negotiate continue")
		return

	case sspi.StatusOK:
		// Authentication complete - extract username and check admin
		if err := sspi.CompleteSession(sess, ctxOut); err != nil {
			slog.Warn("Negotiate: CompleteSession failed", "ip", clientIP, "error", err)
			store.Remove(connID)
			writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Failed to complete authentication")
			return
		}

		if !sess.IsAdmin {
			slog.Warn("Negotiate: non-admin access denied", "ip", clientIP, "user", sess.Username)
			store.Remove(connID)
			writeError(w, http.StatusForbidden, "FORBIDDEN", "Access denied: not a member of BUILTIN\\Administrators")
			return
		}

		slog.Info("Negotiate: authenticated", "ip", clientIP, "user", sess.Username)

		// Include final Negotiate token in response if present
		if len(outputToken) > 0 {
			w.Header().Set("WWW-Authenticate", "Negotiate "+base64.StdEncoding.EncodeToString(outputToken))
		}

		next.ServeHTTP(w, withAuthContext(r, sess.Username, clientIP))
		return

	default:
		slog.Warn("Negotiate: unexpected status", "ip", clientIP, "status", fmt.Sprintf("0x%08X", uint32(status)))
		store.Remove(connID)
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Negotiate authentication failed")
	}
}

// connIDFromRequest extracts the connection ID from the request context,
// falling back to RemoteAddr.
func connIDFromRequest(r *http.Request) string {
	if id, ok := r.Context().Value(ctxKeyConnID).(string); ok && id != "" {
		return id
	}
	return r.RemoteAddr
}

func isWebSocketUpgrade(r *http.Request) bool {
	return strings.EqualFold(r.Header.Get("Upgrade"), "websocket")
}

// isLoopback returns true if the IP is a loopback address (127.0.0.1, ::1).
func isLoopback(ip string) bool {
	switch ip {
	case "127.0.0.1", "::1":
		return true
	}
	return false
}
