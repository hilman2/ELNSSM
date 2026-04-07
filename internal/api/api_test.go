package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hilman2/ELNSSM/internal/config"
	"golang.org/x/crypto/bcrypt"
)

func TestWriteJSON(t *testing.T) {
	w := httptest.NewRecorder()
	writeJSON(w, http.StatusOK, map[string]string{"key": "value"})

	if w.Code != 200 {
		t.Errorf("status = %d, want 200", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}
	body := w.Body.String()
	if body == "" {
		t.Error("body should not be empty")
	}
}

func TestWriteJSONList(t *testing.T) {
	w := httptest.NewRecorder()
	writeJSONList(w, []string{"a", "b"}, 2)

	if w.Code != 200 {
		t.Errorf("status = %d, want 200", w.Code)
	}
}

func TestWriteError(t *testing.T) {
	w := httptest.NewRecorder()
	writeError(w, http.StatusNotFound, "NOT_FOUND", "thing not found")

	if w.Code != 404 {
		t.Errorf("status = %d, want 404", w.Code)
	}
}

func TestExtractIP_RemoteAddr(t *testing.T) {
	r := httptest.NewRequest("GET", "/", nil)
	r.RemoteAddr = "192.168.1.1:12345"
	ip := extractIP(r)
	if ip != "192.168.1.1" {
		t.Errorf("IP = %q, want 192.168.1.1", ip)
	}
}

func TestExtractIP_IgnoresXForwardedFor(t *testing.T) {
	r := httptest.NewRequest("GET", "/", nil)
	r.RemoteAddr = "192.168.1.1:12345"
	r.Header.Set("X-Forwarded-For", "10.0.0.1, 10.0.0.2")
	ip := extractIP(r)
	if ip != "192.168.1.1" {
		t.Errorf("IP = %q, want 192.168.1.1 (X-Forwarded-For should be ignored)", ip)
	}
}

func TestExtractIP_IgnoresXRealIP(t *testing.T) {
	r := httptest.NewRequest("GET", "/", nil)
	r.RemoteAddr = "192.168.1.1:12345"
	r.Header.Set("X-Real-IP", "172.16.0.1")
	ip := extractIP(r)
	if ip != "192.168.1.1" {
		t.Errorf("IP = %q, want 192.168.1.1 (X-Real-IP should be ignored)", ip)
	}
}

func TestIPWhitelist_Allowed(t *testing.T) {
	cfg := &config.APIConfig{IPWhitelist: []string{"127.0.0.1"}}
	handler := ipWhitelist(cfg)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	}))

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/", nil)
	r.RemoteAddr = "127.0.0.1:12345"
	handler.ServeHTTP(w, r)

	if w.Code != 200 {
		t.Errorf("status = %d, want 200", w.Code)
	}
}

func TestIPWhitelist_Blocked(t *testing.T) {
	cfg := &config.APIConfig{IPWhitelist: []string{"127.0.0.1"}}
	handler := ipWhitelist(cfg)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	}))

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/", nil)
	r.RemoteAddr = "10.0.0.1:12345"
	handler.ServeHTTP(w, r)

	if w.Code != 403 {
		t.Errorf("status = %d, want 403", w.Code)
	}
}

func TestIPWhitelist_EmptyDeniesAll(t *testing.T) {
	cfg := &config.APIConfig{IPWhitelist: []string{}}
	handler := ipWhitelist(cfg)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	}))

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/", nil)
	r.RemoteAddr = "8.8.8.8:12345"
	handler.ServeHTTP(w, r)

	if w.Code != 403 {
		t.Errorf("status = %d, want 403 (empty whitelist should deny all)", w.Code)
	}
}

func TestSecurityHeaders(t *testing.T) {
	handler := securityHeaders(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	}))

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/", nil)
	handler.ServeHTTP(w, r)

	if w.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Error("missing X-Content-Type-Options header")
	}
	if w.Header().Get("X-Frame-Options") != "DENY" {
		t.Error("missing X-Frame-Options header")
	}
	if w.Header().Get("Content-Security-Policy") == "" {
		t.Error("missing Content-Security-Policy header")
	}
}

func TestAuthMiddleware_Disabled(t *testing.T) {
	cfg := &config.AuthConfig{Enabled: false}
	handler := authMiddleware(cfg, nil)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	}))

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/", nil)
	r.RemoteAddr = "127.0.0.1:12345"
	handler.ServeHTTP(w, r)

	if w.Code != 200 {
		t.Errorf("status = %d, want 200 (auth disabled)", w.Code)
	}
}

func TestAuthMiddleware_ValidBearerToken(t *testing.T) {
	token := "test-secret-token"
	hash, _ := bcrypt.GenerateFromPassword([]byte(token), bcrypt.MinCost)
	cfg := &config.AuthConfig{Enabled: true, Type: "token", TokenHash: string(hash)}

	handler := authMiddleware(cfg, nil)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	}))

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/", nil)
	r.RemoteAddr = "192.168.1.10:12345"
	r.Header.Set("Authorization", "Bearer "+token)
	handler.ServeHTTP(w, r)

	if w.Code != 200 {
		t.Errorf("status = %d, want 200", w.Code)
	}
}

func TestAuthMiddleware_InvalidBearerToken(t *testing.T) {
	hash, _ := bcrypt.GenerateFromPassword([]byte("correct-token"), bcrypt.MinCost)
	cfg := &config.AuthConfig{Enabled: true, Type: "token", TokenHash: string(hash)}

	handler := authMiddleware(cfg, nil)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	}))

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/", nil)
	r.RemoteAddr = "192.168.1.10:12345"
	r.Header.Set("Authorization", "Bearer wrong-token")
	handler.ServeHTTP(w, r)

	if w.Code != 401 {
		t.Errorf("status = %d, want 401", w.Code)
	}
}

func TestAuthMiddleware_MissingAuth(t *testing.T) {
	hash, _ := bcrypt.GenerateFromPassword([]byte("token"), bcrypt.MinCost)
	cfg := &config.AuthConfig{Enabled: true, Type: "token", TokenHash: string(hash)}

	handler := authMiddleware(cfg, nil)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	}))

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/", nil)
	r.RemoteAddr = "192.168.1.10:12345"
	handler.ServeHTTP(w, r)

	if w.Code != 401 {
		t.Errorf("status = %d, want 401", w.Code)
	}
	if w.Header().Get("WWW-Authenticate") == "" {
		t.Error("missing WWW-Authenticate header")
	}
}

func TestAuthMiddleware_ValidBasicAuth(t *testing.T) {
	passHash, _ := bcrypt.GenerateFromPassword([]byte("secret"), bcrypt.MinCost)
	cfg := &config.AuthConfig{Enabled: true, Type: "basic", Username: "admin", PasswordHash: string(passHash)}

	handler := authMiddleware(cfg, nil)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	}))

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/", nil)
	r.RemoteAddr = "192.168.1.10:12345"
	r.SetBasicAuth("admin", "secret")
	handler.ServeHTTP(w, r)

	if w.Code != 200 {
		t.Errorf("status = %d, want 200", w.Code)
	}
}

func TestAuthMiddleware_InvalidBasicAuth(t *testing.T) {
	passHash, _ := bcrypt.GenerateFromPassword([]byte("secret"), bcrypt.MinCost)
	cfg := &config.AuthConfig{Enabled: true, Type: "basic", Username: "admin", PasswordHash: string(passHash)}

	handler := authMiddleware(cfg, nil)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	}))

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/", nil)
	r.RemoteAddr = "192.168.1.10:12345"
	r.SetBasicAuth("admin", "wrong")
	handler.ServeHTTP(w, r)

	if w.Code != 401 {
		t.Errorf("status = %d, want 401", w.Code)
	}
}

func TestAuthMiddleware_WebSocketQueryToken(t *testing.T) {
	token := "ws-secret"
	hash, _ := bcrypt.GenerateFromPassword([]byte(token), bcrypt.MinCost)
	cfg := &config.AuthConfig{Enabled: true, Type: "token", TokenHash: string(hash)}

	handler := authMiddleware(cfg, nil)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	}))

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/?token="+token, nil)
	r.RemoteAddr = "192.168.1.10:12345"
	r.Header.Set("Upgrade", "websocket")
	r.Header.Set("Connection", "Upgrade")
	handler.ServeHTTP(w, r)

	if w.Code != 200 {
		t.Errorf("status = %d, want 200", w.Code)
	}
}

func TestValidateServiceID_Valid(t *testing.T) {
	valid := []string{"my-app", "app.v2", "App_123", "a", "a-b.c_d"}
	for _, id := range valid {
		w := httptest.NewRecorder()
		if !validateServiceID(w, id) {
			t.Errorf("validateServiceID(%q) = false, want true", id)
		}
	}
}

func TestValidateServiceID_Invalid(t *testing.T) {
	invalid := []string{"", "../etc/passwd", "-starts-with-dash", ".starts-with-dot", "has spaces", "has/slash", "a&b"}
	for _, id := range invalid {
		w := httptest.NewRecorder()
		if validateServiceID(w, id) {
			t.Errorf("validateServiceID(%q) = true, want false", id)
		}
		if w.Code != 400 {
			t.Errorf("validateServiceID(%q) status = %d, want 400", id, w.Code)
		}
	}
}

func TestValidateLogStream_Valid(t *testing.T) {
	for _, s := range []string{"stdout", "stderr", "combined"} {
		w := httptest.NewRecorder()
		if !validateLogStream(w, s) {
			t.Errorf("validateLogStream(%q) = false, want true", s)
		}
	}
}

func TestValidateLogStream_Invalid(t *testing.T) {
	invalid := []string{"../../windows/system32", "other", "", "stdin"}
	for _, s := range invalid {
		w := httptest.NewRecorder()
		if validateLogStream(w, s) {
			t.Errorf("validateLogStream(%q) = true, want false", s)
		}
		if w.Code != 400 {
			t.Errorf("validateLogStream(%q) status = %d, want 400", s, w.Code)
		}
	}
}

func TestContainsShellMeta(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"normal-command", false},
		{"cmd /c dir", false},
		{"cmd & echo pwned", true},
		{"test; rm -rf /", true},
		{"$(evil)", true},
		{"test\necho", true},
		{"pipe|chain", true},
	}
	for _, tt := range tests {
		got := containsShellMeta(tt.input)
		if got != tt.want {
			t.Errorf("containsShellMeta(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}

func TestRateLimiter(t *testing.T) {
	rl := NewRateLimiter(3, 1*60*1e9) // 3 requests per minute
	ip := "192.168.1.1"

	for i := 0; i < 3; i++ {
		if !rl.Allow(ip) {
			t.Errorf("request %d should be allowed", i+1)
		}
	}

	if rl.Allow(ip) {
		t.Error("4th request should be rate limited")
	}

	// Different IP should still be allowed
	if !rl.Allow("10.0.0.1") {
		t.Error("different IP should be allowed")
	}
}
