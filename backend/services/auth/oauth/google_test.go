package oauth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Sephy314/chinwag/backend/services/auth/shared/logger"
	"github.com/labstack/echo/v5"
)

const testJkt = "V4l2h3eM_94YgJXfXzYhZKcXK4gMl8TqH4eV1kN2B_o"

type mockCache struct {
	set map[string]string
}

func newMockCache() *mockCache {
	return &mockCache{set: map[string]string{}}
}

func (m *mockCache) Get(_ context.Context, key string) (string, error) {
	return m.set[key], nil
}

func (m *mockCache) Set(_ context.Context, key string, value any, _ time.Duration) error {
	m.set[key] = value.(string)
	return nil
}

func (m *mockCache) SetNX(_ context.Context, key string, value any, _ time.Duration) (bool, error) {
	if _, ok := m.set[key]; ok {
		return false, nil
	}
	m.set[key] = value.(string)
	return true, nil
}

func (m *mockCache) Delete(_ context.Context, key string) error {
	delete(m.set, key)
	return nil
}

func (m *mockCache) TTL(_ context.Context, key string) (time.Duration, error) {
	return 0, nil
}

func (m *mockCache) HSet(_ context.Context, key string, _ map[string]string, _ time.Duration) error {
	return nil
}

func (m *mockCache) HGetAll(_ context.Context, _ string) (map[string]string, error) {
	return nil, nil
}

func (m *mockCache) SAdd(_ context.Context, _ string, _ time.Duration, _ ...string) error {
	return nil
}

func (m *mockCache) SMembers(_ context.Context, _ string) ([]string, error) {
	return nil, nil
}

func (m *mockCache) Eval(_ context.Context, _ string, keys []string, _ ...any) (any, error) {
	v, ok := m.set[keys[0]]
	if ok {
		delete(m.set, keys[0])
	}
	return v, nil
}

func (m *mockCache) AcquireLock(_ context.Context, _ string, _ string, _ time.Duration) (bool, error) {
	return true, nil
}

func (m *mockCache) ReleaseLock(_ context.Context, _ string, _ string) error {
	return nil
}

func (m *mockCache) ConsumeNonce(_ context.Context, nonce string) (bool, error) {
	if _, ok := m.set["dpop:nonce:"+nonce]; !ok {
		return false, nil
	}
	delete(m.set, "dpop:nonce:"+nonce)
	return true, nil
}

func (m *mockCache) ReserveJti(_ context.Context, jti string, _ time.Duration) (bool, error) {
	key := "dpop:jti:" + jti
	if _, ok := m.set[key]; ok {
		return false, nil
	}
	m.set[key] = "1"
	return true, nil
}

type noopLogger struct{}

func (noopLogger) Info(_ string, _ ...any)   {}
func (noopLogger) Error(_ string, _ ...any)  {}
func (noopLogger) Debug(_ string, _ ...any)  {}
func (noopLogger) Warn(_ string, _ ...any)   {}
func (noopLogger) Fatal(_ string, _ ...any)  {}
func (noopLogger) With(_ ...any) logger.Logger { return noopLogger{} }

func newTestLogger() logger.Logger { return noopLogger{} }

func TestIsValidJkt(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want bool
	}{
		{"valid thumbprint", testJkt, true},
		{"empty", "", false},
		{"too short", "abc", false},
		{"too long", testJkt + "_extra", false},
		{"url chars not allowed", "V4l2h3eM/94YgJXfXzYhZKcXK4gMl8TqH4eV1kN2B_o", false},
		{"padding not allowed", "V4l2h3eM_94YgJXfXzYhZKcXK4gMl8TqH4eV1kN2B_o=", false},
		{"invalid char", "V4l2h3eM_94YgJXfXzYhZKcXK4gMl8TqH4eV1kN2B@o", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isValidJkt(tt.in); got != tt.want {
				t.Errorf("isValidJkt(%q) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}

func TestHandleRedirectRejectsMissingJkt(t *testing.T) {
	h := &GoogleOAuthHandler{FrontendURL: "http://localhost:3000", Cache: newMockCache(), Config: &GoogleConfig{ClientID: "cid", RedirectURL: "http://localhost:8000/auth/google/callback"}, log: newTestLogger()}

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/auth/google", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	if err := h.HandleRedirect(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusTemporaryRedirect {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusTemporaryRedirect)
	}
	if loc := rec.Header().Get("Location"); loc != "http://localhost:3000/login?error=oauth_binding_required" {
		t.Fatalf("location = %q", loc)
	}
}

func TestHandleRedirectRejectsMalformedJkt(t *testing.T) {
	h := &GoogleOAuthHandler{FrontendURL: "http://localhost:3000", Cache: newMockCache(), Config: &GoogleConfig{ClientID: "cid", RedirectURL: "http://localhost:8000/auth/google/callback"}, log: newTestLogger()}

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/auth/google?jkt=not-a-thumbprint", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	if err := h.HandleRedirect(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusTemporaryRedirect {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusTemporaryRedirect)
	}
	if loc := rec.Header().Get("Location"); loc != "http://localhost:3000/login?error=oauth_binding_required" {
		t.Fatalf("location = %q", loc)
	}
}

func TestHandleRedirectStoresBinding(t *testing.T) {
	mc := newMockCache()
	h := &GoogleOAuthHandler{FrontendURL: "http://localhost:3000", Cache: mc, Config: &GoogleConfig{ClientID: "cid", RedirectURL: "http://localhost:8000/auth/google/callback"}, log: newTestLogger()}

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/auth/google?jkt="+testJkt, nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	if err := h.HandleRedirect(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusTemporaryRedirect {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusTemporaryRedirect)
	}
	if loc := rec.Header().Get("Location"); loc == "http://localhost:3000/login?error=oauth_binding_required" {
		t.Fatalf("redirected to error page with valid jkt: %q", loc)
	}
	if len(mc.set) != 1 {
		t.Fatalf("expected one stored state binding, got %d", len(mc.set))
	}
	for k, v := range mc.set {
		if v != testJkt {
			t.Errorf("stored binding %q for key %s, want %q", v, k, testJkt)
		}
	}
}
