// Package sspi provides Windows SSPI Negotiate / NTLM authentication
// for the ELNSSM API server. It binds the Win32 secur32.dll surface
// needed to accept clients via Integrated Windows Authentication.
package sspi

import (
	"log/slog"
	"sync"
	"time"
)

// SessionState tracks the SSPI handshake progress for a connection.
type SessionState int

const (
	StateNew       SessionState = iota // No handshake started
	StateChallenge                     // Waiting for client to respond to challenge
	StateComplete                      // Authentication complete
)

// Session holds SSPI state for a single TCP connection.
type Session struct {
	CtxHandle *SecHandle
	State     SessionState
	Username  string
	IsAdmin   bool
	CreatedAt time.Time
	TouchedAt time.Time
}

// SessionStore manages per-connection SSPI sessions with a shared server credential.
type SessionStore struct {
	mu       sync.Mutex
	sessions map[string]*Session
	cred     *SecHandle
	done     chan struct{}
}

const (
	cleanupInterval = 30 * time.Second
	staleDuration   = 60 * time.Second
)

// NewSessionStore acquires server credentials for the "Negotiate" package and
// starts a background goroutine for TTL cleanup of stale sessions.
func NewSessionStore() (*SessionStore, error) {
	cred, err := AcquireCredentialsHandle()
	if err != nil {
		return nil, err
	}

	ss := &SessionStore{
		sessions: make(map[string]*Session),
		cred:     cred,
		done:     make(chan struct{}),
	}

	go ss.cleanupLoop()
	return ss, nil
}

// Cred returns the shared server credential handle.
func (ss *SessionStore) Cred() *SecHandle {
	return ss.cred
}

// GetOrCreate returns the session for the given connection ID, creating one if needed.
func (ss *SessionStore) GetOrCreate(connID string) *Session {
	ss.mu.Lock()
	defer ss.mu.Unlock()

	if s, ok := ss.sessions[connID]; ok {
		s.TouchedAt = time.Now()
		return s
	}

	s := &Session{
		State:     StateNew,
		CreatedAt: time.Now(),
		TouchedAt: time.Now(),
	}
	ss.sessions[connID] = s
	return s
}

// Get returns the session for the given connection ID, or nil if not found.
func (ss *SessionStore) Get(connID string) *Session {
	ss.mu.Lock()
	defer ss.mu.Unlock()

	if s, ok := ss.sessions[connID]; ok {
		s.TouchedAt = time.Now()
		return s
	}
	return nil
}

// Remove deletes the session for the given connection ID and cleans up its SSPI context.
func (ss *SessionStore) Remove(connID string) {
	ss.mu.Lock()
	s, ok := ss.sessions[connID]
	if ok {
		delete(ss.sessions, connID)
	}
	ss.mu.Unlock()

	if ok && s.CtxHandle != nil {
		DeleteSecurityContext(s.CtxHandle)
	}
}

// Close stops the cleanup goroutine and releases all resources.
func (ss *SessionStore) Close() {
	close(ss.done)

	ss.mu.Lock()
	for id, s := range ss.sessions {
		if s.CtxHandle != nil {
			DeleteSecurityContext(s.CtxHandle)
		}
		delete(ss.sessions, id)
	}
	ss.mu.Unlock()

	FreeCredentialsHandle(ss.cred)
}

// cleanupLoop periodically removes stale sessions.
func (ss *SessionStore) cleanupLoop() {
	ticker := time.NewTicker(cleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ss.done:
			return
		case <-ticker.C:
			ss.evictStale()
		}
	}
}

func (ss *SessionStore) evictStale() {
	now := time.Now()
	var stale []string

	ss.mu.Lock()
	for id, s := range ss.sessions {
		// Only evict incomplete sessions (completed sessions live until connection close)
		if s.State != StateComplete && now.Sub(s.TouchedAt) > staleDuration {
			stale = append(stale, id)
		}
	}
	// Remove inside the same lock to avoid double-free
	toClean := make([]*SecHandle, 0, len(stale))
	for _, id := range stale {
		if s, ok := ss.sessions[id]; ok {
			if s.CtxHandle != nil {
				toClean = append(toClean, s.CtxHandle)
			}
			delete(ss.sessions, id)
		}
	}
	ss.mu.Unlock()

	// Clean up SSPI handles outside the lock
	for _, ctx := range toClean {
		DeleteSecurityContext(ctx)
	}

	if len(stale) > 0 {
		slog.Debug("SSPI session cleanup", "evicted", len(stale))
	}
}

// CompleteSession marks a session as authenticated with the given username and admin status.
// It also closes the impersonation token after the admin check.
func CompleteSession(sess *Session, ctx *SecHandle) error {
	username, err := QueryContextNames(ctx)
	if err != nil {
		return err
	}

	token, err := QuerySecurityContextToken(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = token.Close() }()

	admin, err := IsAdmin(token)
	if err != nil {
		// Log but don't fail - default to non-admin
		slog.Warn("SSPI admin check failed", "user", username, "error", err)
		admin = false
	}

	sess.CtxHandle = ctx
	sess.State = StateComplete
	sess.Username = username
	sess.IsAdmin = admin
	sess.TouchedAt = time.Now()

	return nil
}
