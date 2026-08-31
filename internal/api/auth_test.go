package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"golang.org/x/crypto/bcrypt"

	"github.com/hilman2/ELNSSM/internal/config"
)

func okHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
}

func TestAuthMiddleware_LocalBypassAllowed(t *testing.T) {
	cfg := &config.AuthConfig{Enabled: true, Type: "token", AllowLocalBypass: true}
	handler := authMiddleware(cfg, nil)(okHandler())

	for _, addr := range []string{"127.0.0.1:12345", "[::1]:12345"} {
		w := httptest.NewRecorder()
		r := httptest.NewRequest("GET", "/", http.NoBody)
		r.RemoteAddr = addr
		handler.ServeHTTP(w, r)

		if w.Code != http.StatusOK {
			t.Errorf("RemoteAddr %s: status = %d, want 200 with the bypass enabled", addr, w.Code)
		}
	}
}

// With the bypass switched off, a loopback caller is just another client and
// has to authenticate. This is the case that matters on an RDS host, where
// ordinary users are signed in, and for a service account that reaches
// loopback without any interactive logon at all.
func TestAuthMiddleware_LocalBypassDisabled(t *testing.T) {
	hash, err := bcrypt.GenerateFromPassword([]byte("s3cret"), bcrypt.MinCost)
	if err != nil {
		t.Fatalf("hashing token: %v", err)
	}
	cfg := &config.AuthConfig{
		Enabled:          true,
		Type:             "token",
		TokenHash:        string(hash),
		AllowLocalBypass: false,
	}
	handler := authMiddleware(cfg, nil)(okHandler())

	t.Run("no credentials is rejected", func(t *testing.T) {
		w := httptest.NewRecorder()
		r := httptest.NewRequest("GET", "/", http.NoBody)
		r.RemoteAddr = "127.0.0.1:12345"
		handler.ServeHTTP(w, r)

		if w.Code != http.StatusUnauthorized {
			t.Errorf("status = %d, want 401 with the bypass disabled", w.Code)
		}
	})

	t.Run("valid token is accepted", func(t *testing.T) {
		w := httptest.NewRecorder()
		r := httptest.NewRequest("GET", "/", http.NoBody)
		r.RemoteAddr = "127.0.0.1:12345"
		r.Header.Set("Authorization", "Bearer s3cret")
		handler.ServeHTTP(w, r)

		if w.Code != http.StatusOK {
			t.Errorf("status = %d, want 200 for a loopback caller with a valid token", w.Code)
		}
	})
}

// The bypass must not depend on a header, or it could be claimed remotely.
func TestAuthMiddleware_LocalBypassIgnoresForwardedFor(t *testing.T) {
	cfg := &config.AuthConfig{Enabled: true, Type: "token", AllowLocalBypass: true}
	handler := authMiddleware(cfg, nil)(okHandler())

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/", http.NoBody)
	r.RemoteAddr = "203.0.113.7:12345"
	r.Header.Set("X-Forwarded-For", "127.0.0.1")
	handler.ServeHTTP(w, r)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401: a spoofed X-Forwarded-For must not reach the bypass", w.Code)
	}
}

// DefaultConfig has to keep the bypass on, so upgrading does not lock an
// operator out of a Guardian they administer from the console.
func TestDefaultConfigKeepsLocalBypass(t *testing.T) {
	if !config.DefaultConfig().API.Auth.AllowLocalBypass {
		t.Error("AllowLocalBypass = false in DefaultConfig, want true to preserve existing behaviour")
	}
}
