package middleware

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/labstack/echo/v5"
	"github.com/labstack/echo/v5/middleware"
)

// rateLimitSkipper excludes endpoints that must never be rate-limited: the
// liveness/readiness probes and the Prometheus scrape endpoint (probes and
// scrapes would otherwise consume the budget and/or get throttled).
func rateLimitSkipper(c *echo.Context) bool {
	switch c.Request().URL.Path {
	case "/health", "/metrics":
		return true
	}
	return false
}

// RateLimit returns a per-client-IP token-bucket rate limiter. The gateway is
// the single edge for all HTTP traffic (behind Traefik), so this throttles
// abusive clients before they reach the backend services. Requests over the
// limit get a 429 Too Many Requests. When disabled or given a non-positive
// rate/burst it is a no-op (so local dev can turn it off easily).
func RateLimit(enabled bool, rate float64, burst int) echo.MiddlewareFunc {
	if !enabled || rate <= 0 || burst <= 0 {
		return func(next echo.HandlerFunc) echo.HandlerFunc {
			return func(c *echo.Context) error {
				return next(c)
			}
		}
	}

	store := middleware.NewRateLimiterMemoryStoreWithConfig(
		middleware.RateLimiterMemoryStoreConfig{
			Rate:      rate,
			Burst:     burst,
			ExpiresIn: 5 * time.Minute,
		},
	)

	return middleware.RateLimiterWithConfig(middleware.RateLimiterConfig{
		Skipper: rateLimitSkipper,
		IdentifierExtractor: func(c *echo.Context) (string, error) {
			return c.RealIP(), nil
		},
		Store: store,
		DenyHandler: func(c *echo.Context, identifier string, _ error) error {
			// Runs in the Pre chain, before the AccessLogger — log here so
			// rate-limited requests are still visible.
			slog.Warn("rate limit exceeded",
				"client_ip", identifier,
				"method", c.Request().Method,
				"path", c.Request().URL.Path,
			)
			return c.JSON(http.StatusTooManyRequests, map[string]string{
				"error": "rate limit exceeded",
			})
		},
	})
}
