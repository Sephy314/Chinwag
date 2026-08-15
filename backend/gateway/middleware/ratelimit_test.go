package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v5"
)

func doGet(e *echo.Echo, path string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, path, nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	return rec
}

func TestRateLimitDeniesBeyondBurst(t *testing.T) {
	e := echo.New()
	e.Use(RateLimit(true, 100, 3))
	e.GET("/", func(c *echo.Context) error { return c.String(http.StatusOK, "ok") })

	// Burst = 3 → the first three requests pass.
	for i := 0; i < 3; i++ {
		if rec := doGet(e, "/"); rec.Code != http.StatusOK {
			t.Fatalf("request %d: got %d, want 200", i+1, rec.Code)
		}
	}
	// The fourth exceeds the burst (no refill in a fast loop) → 429.
	if rec := doGet(e, "/"); rec.Code != http.StatusTooManyRequests {
		t.Fatalf("4th request: got %d, want 429", rec.Code)
	}
}

func TestRateLimitSkipsHealthAndMetrics(t *testing.T) {
	e := echo.New()
	e.Use(RateLimit(true, 100, 1)) // burst 1 → only one non-skipped request fits
	e.GET("/", func(c *echo.Context) error { return c.String(http.StatusOK, "ok") })
	e.GET("/health", func(c *echo.Context) error { return c.String(http.StatusOK, "ok") })
	e.GET("/metrics", func(c *echo.Context) error { return c.String(http.StatusOK, "ok") })

	// Probes / scrape are never rate-limited, even beyond the burst.
	for i := 0; i < 5; i++ {
		if rec := doGet(e, "/health"); rec.Code != http.StatusOK {
			t.Fatalf("health #%d: got %d, want 200 (skipped)", i+1, rec.Code)
		}
		if rec := doGet(e, "/metrics"); rec.Code != http.StatusOK {
			t.Fatalf("metrics #%d: got %d, want 200 (skipped)", i+1, rec.Code)
		}
	}
	// Normal routes share the per-IP budget.
	if rec := doGet(e, "/"); rec.Code != http.StatusOK {
		t.Fatalf("first /: got %d, want 200", rec.Code)
	}
	if rec := doGet(e, "/"); rec.Code != http.StatusTooManyRequests {
		t.Fatalf("second /: got %d, want 429", rec.Code)
	}
}

func TestRateLimitDisabledIsNoop(t *testing.T) {
	e := echo.New()
	e.Use(RateLimit(false, 0, 0))
	e.GET("/", func(c *echo.Context) error { return c.String(http.StatusOK, "ok") })

	for i := 0; i < 25; i++ {
		if rec := doGet(e, "/"); rec.Code != http.StatusOK {
			t.Fatalf("request %d: got %d, want 200", i+1, rec.Code)
		}
	}
}

// TestRateLimitFullChain mirrors main.go's composition: XFF IP extractor, the
// Use chain (RequestID, AccessLogger, Metrics), RateLimit as a Pre middleware
// registered BEFORE a short-circuiting Pre proxy (like setupRoutes in proxy.go).
// It proves the limiter throttles proxied routes too — the proxy must not be
// able to bypass it, and keys stay per real client IP via XFF.
func TestRateLimitFullChain(t *testing.T) {
	e := echo.New()
	e.IPExtractor = echo.ExtractIPFromXFFHeader()
	e.Use(RequestID())
	e.Use(AccessLogger())
	e.Use(MetricsMiddleware())
	// Limiter first in the Pre chain — before the short-circuiting proxy.
	e.Pre(RateLimit(true, 0.001, 2))
	// Simulate setupRoutes: a Pre proxy that handles /chat directly and returns,
	// bypassing the Use chain (this is what made a Use-only limiter ineffective).
	e.Pre(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) error {
			if c.Request().URL.Path == "/chat" {
				return c.String(http.StatusServiceUnavailable, "proxy-unavailable")
			}
			return next(c)
		}
	})
	e.GET("/chat", func(c *echo.Context) error { return c.String(http.StatusOK, "ok") })

	do := func(ip string) int {
		req := httptest.NewRequest(http.MethodGet, "/chat", nil)
		req.Header.Set("X-Forwarded-For", ip)
		// Simulate the connection coming from the Traefik pod (private IP,
		// trusted by ExtractIPFromXFFHeader), so the XFF client IP is used as
		// the rate-limit key — exactly what happens behind the ingress.
		req.RemoteAddr = "10.42.0.1:1234"
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		return rec.Code
	}

	// Burst = 2 per IP: same IP reaches the proxy twice, then the limiter
	// (running before the proxy) denies with 429.
	for i := 0; i < 2; i++ {
		if got := do("203.0.113.5"); got != http.StatusServiceUnavailable {
			t.Fatalf("request %d (same IP): got %d, want 503 from proxy", i+1, got)
		}
	}
	if got := do("203.0.113.5"); got != http.StatusTooManyRequests {
		t.Fatalf("3rd request (same IP): got %d, want 429 from limiter", got)
	}
	// A different IP has its own budget and reaches the proxy.
	if got := do("198.51.100.9"); got != http.StatusServiceUnavailable {
		t.Fatalf("different IP: got %d, want 503 (own budget)", got)
	}
}
