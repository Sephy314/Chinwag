package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v5"
	"github.com/prometheus/client_golang/prometheus/testutil"
)

func TestServiceForPath(t *testing.T) {
	cases := map[string]string{
		"/auth/login":      "auth",
		"/auth":            "auth",
		"/author":          "gateway", // prefix must be a path boundary
		"/authx":           "gateway",
		"/rooms":           "rooms",
		"/rooms/1":         "rooms",
		"/roomship":        "gateway",
		"/users/me":        "users",
		"/chat/rooms/x/ws": "chat",
		"/chatty":          "gateway",
		"/admin/rooms":     "admin",
		"/admin":           "admin",
		"/adminpanel":      "gateway",
		"/health":          "health",
		"/metrics":         "metrics",
		"/":                "gateway",
		"/unknown":         "gateway",
	}
	for path, want := range cases {
		if got := serviceForPath(path); got != want {
			t.Errorf("serviceForPath(%q) = %q, want %q", path, got, want)
		}
	}
}

func TestMetricsMiddlewareRecordsRequest(t *testing.T) {
	e := echo.New()
	e.Use(MetricsMiddleware())
	e.GET("/health", func(c *echo.Context) error {
		return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
	})

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected status %d", rec.Code)
	}

	got := testutil.ToFloat64(httpRequestsTotal.WithLabelValues("health", "GET", "200"))
	if got != 1 {
		t.Errorf("http_requests_total{service=health,method=GET,code=200} = %v, want 1", got)
	}
}

func TestMetricsMiddlewareRecords5xx(t *testing.T) {
	e := echo.New()
	e.Use(MetricsMiddleware())
	e.GET("/auth/boom", func(c *echo.Context) error {
		return c.String(http.StatusInternalServerError, "boom")
	})

	req := httptest.NewRequest(http.MethodGet, "/auth/boom", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	got := testutil.ToFloat64(httpRequestsTotal.WithLabelValues("auth", "GET", "500"))
	if got != 1 {
		t.Errorf("http_requests_total{service=auth,code=500} = %v, want 1", got)
	}
}

func TestMetricsMiddlewareSkipsMetricsPath(t *testing.T) {
	e := echo.New()
	e.Use(MetricsMiddleware())
	e.GET("/metrics", func(c *echo.Context) error {
		return c.String(http.StatusOK, "ok")
	})

	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected status %d", rec.Code)
	}

	// The Prometheus scrape request must not be recorded as application traffic.
	if got := testutil.ToFloat64(httpRequestsTotal.WithLabelValues("metrics", "GET", "200")); got != 0 {
		t.Errorf("http_requests_total{service=metrics,code=200} = %v, want 0 (scrape excluded)", got)
	}
}
