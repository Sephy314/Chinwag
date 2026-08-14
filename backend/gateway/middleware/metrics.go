package middleware

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/labstack/echo/v5"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	httpRequestsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "http_requests_total",
		Help: "Total number of HTTP requests handled by the gateway.",
	}, []string{"service", "method", "code"})

	httpRequestDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "http_request_duration_seconds",
		Help:    "HTTP request latency in seconds.",
		Buckets: []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10},
	}, []string{"service"})
)

// metricsPath is the Prometheus scrape endpoint. Scrape requests to it are
// excluded from the application metrics so they never pollute
// http_requests_total / latency-based alerts.
const metricsPath = "/metrics"

// MetricsMiddleware records every request into the Prometheus metrics above.
// The `service` label is derived from the gateway route prefix (see
// proxy.go), so each proxied backend (auth/room/chat/admin) gets its own
// request / error / traffic series. Prometheus's own /metrics scrape requests
// are skipped so they are not counted as application traffic.
func MetricsMiddleware() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) error {
			// Exclude Prometheus scrape requests from application metrics.
			if c.Request().URL.Path == metricsPath {
				return next(c)
			}

			start := time.Now()

			err := next(c)

			status := 0
			if r, rErr := echo.UnwrapResponse(c.Response()); rErr == nil {
				status = r.Status
			}
			if status == 0 {
				status = http.StatusInternalServerError
			}

			svc := serviceForPath(c.Request().URL.Path)
			httpRequestsTotal.WithLabelValues(svc, c.Request().Method, strconv.Itoa(status)).Inc()
			httpRequestDuration.WithLabelValues(svc).Observe(time.Since(start).Seconds())

			return err
		}
	}
}

// MetricsHandler exposes the Prometheus metrics on GET /metrics. It is
// intentionally NOT routed through the public ingress — Prometheus scrapes it
// over the cluster network via the gateway Service annotations.
func MetricsHandler() echo.HandlerFunc {
	h := promhttp.Handler()
	return func(c *echo.Context) error {
		h.ServeHTTP(c.Response(), c.Request())
		return nil
	}
}

// serviceForPath maps the gateway's path prefixes to a logical service name.
// Unknown paths (and the gateway's own endpoints) are attributed to "gateway".
func serviceForPath(path string) string {
	switch {
	case pathPrefixMatch(path, "/auth"):
		return "auth"
	case pathPrefixMatch(path, "/rooms"):
		return "rooms"
	case pathPrefixMatch(path, "/users"):
		return "users"
	case pathPrefixMatch(path, "/chat"):
		return "chat"
	case pathPrefixMatch(path, "/admin"):
		return "admin"
	case path == "/health":
		return "health"
	case path == metricsPath:
		return "metrics"
	default:
		return "gateway"
	}
}

// pathPrefixMatch reports whether path equals prefix or begins with prefix
// followed by a "/" boundary, so "/auth" matches "/auth" and "/auth/login"
// but NOT "/author". Keeping the boundary check means a future route that
// merely shares a textual prefix (e.g. "/author") is never mislabeled.
func pathPrefixMatch(path, prefix string) bool {
	return path == prefix || strings.HasPrefix(path, prefix+"/")
}
