package middleware

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/labstack/echo/v5"
	"github.com/labstack/echo/v5/middleware"
)

// rateLimitSkipper excludes endpoints that must never be rate-limited: the
// liveness/readiness probes and the Prometheus scrape endpoint (probes and
// scrapes would otherwise consume the budget and/or get throttled), plus CORS
// preflight OPTIONS.
func rateLimitSkipper(c *echo.Context) bool {
	switch {
	case c.Request().URL.Path == "/health", c.Request().URL.Path == "/metrics":
		return true
	case c.Request().Method == http.MethodOptions:
		return true // CORS preflight is handled by the CORS middleware
	}
	return false
}

// RateLimitConfig configures the rate-limiting middleware.
type RateLimitConfig struct {
	Enabled bool
	Rate    float64 // tokens per second per IP
	Burst   int     // max burst per IP

	// Redis is the authoritative distributed limiter. When nil the middleware
	// runs L1-only (memory backend, or Redis not configured).
	Redis Limiter

	// RedisTimeout bounds a single Redis round-trip; on expiry the middleware
	// falls back to L1 (fail-open). Client read/write timeouts also apply.
	RedisTimeout time.Duration

	// Redis circuit breaker: after RedisCircuitThreshold consecutive failures
	// the breaker opens and requests skip the Redis round-trip (immediate
	// L1-only) instead of each paying RedisTimeout; a probe after
	// RedisCircuitCooldown re-checks Redis and closes on success.
	RedisCircuitThreshold int
	RedisCircuitCooldown  time.Duration

	// L1ExpiresIn is how long idle per-IP L1 buckets live before cleanup
	// (lazy initialisation — no key preload, idle state expires).
	L1ExpiresIn time.Duration
}

// deny429 writes the standard 429 response and logs the event. It runs in the
// Pre chain, before the AccessLogger, so the log line is what makes
// rate-limited requests visible.
func deny429(c *echo.Context, identifier string) error {
	slog.Warn("rate limit exceeded",
		"client_ip", identifier,
		"method", c.Request().Method,
		"path", c.Request().URL.Path,
	)
	return c.JSON(http.StatusTooManyRequests, map[string]string{
		"error": "rate limit exceeded",
	})
}

// RateLimit returns a per-client-IP token-bucket limiter.
//
// Normal path (Redis available):
//
//	Request → L1 (cheap local gate) → Redis (authoritative) → backend
//
// L1 rejects obvious local bursts without a Redis round-trip; Redis then makes
// the authoritative allow/deny decision, so multiple gateway replicas share one
// global per-IP budget.
//
// Redis failure path (connection error / timeout / command error): the request
// is NOT failed closed. The L1 decision alone stands, so the service stays
// available with local degraded protection. The next request simply goes back
// through Redis when it recovers — L1 state is never written back to Redis.
//
// NOTE: during an L1-only period (Redis down, or memory backend) the global
// per-IP limit is approximate across replicas — each pod enforces its own local
// budget. Abuse protection is best-effort in degraded mode, by design.
func RateLimit(cfg RateLimitConfig) echo.MiddlewareFunc {
	if !cfg.Enabled || cfg.Rate <= 0 || cfg.Burst <= 0 {
		return func(next echo.HandlerFunc) echo.HandlerFunc {
			return func(c *echo.Context) error {
				return next(c)
			}
		}
	}

	// L1 in-memory limiter: lazy per-IP buckets that expire when idle.
	l1 := middleware.NewRateLimiterMemoryStoreWithConfig(
		middleware.RateLimiterMemoryStoreConfig{
			Rate:      cfg.Rate,
			Burst:     cfg.Burst,
			ExpiresIn: cfg.L1ExpiresIn,
		},
	)

	// Circuit breaker around the Redis call (see circuit_breaker.go).
	var breaker *circuitBreaker
	if cfg.Redis != nil {
		threshold := cfg.RedisCircuitThreshold
		if threshold <= 0 {
			threshold = 3
		}
		cooldown := cfg.RedisCircuitCooldown
		if cooldown <= 0 {
			cooldown = 5 * time.Second
		}
		breaker = newCircuitBreaker(threshold, cooldown)
	}

	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) error {
			if rateLimitSkipper(c) {
				return next(c)
			}

			identifier := c.RealIP()

			// L1 fast path: reject local bursts without a Redis round-trip.
			if l1OK, _ := l1.Allow(identifier); !l1OK {
				rateLimitRejected.WithLabelValues("l1").Inc()
				return deny429(c, identifier)
			}

			// Redis is authoritative when available (behind a circuit breaker:
			// once an outage is detected, requests skip the Redis round-trip
			// instead of each blocking on RedisTimeout).
			if cfg.Redis != nil {
				if !breaker.allow() {
					// Breaker open — degraded L1-only fast path, fail-open.
					rateLimitL1Fallback.Inc()
					rateLimitAllowed.WithLabelValues("l1").Inc()
					return next(c)
				}

				ctx, cancel := context.WithTimeout(c.Request().Context(), cfg.RedisTimeout)
				defer cancel()

				start := time.Now()
				allowed, err := cfg.Redis.Allow(ctx, identifier)
				rateLimitRedisLatency.Observe(time.Since(start).Seconds())

				if err != nil {
					// Degraded mode: Redis unavailable. Do NOT fail closed —
					// the L1 decision above already allowed this request.
					breaker.failure()
					rateLimitRedisErrors.Inc()
					rateLimitL1Fallback.Inc()
					rateLimitAllowed.WithLabelValues("l1").Inc()
					return next(c)
				}
				breaker.success()
				if !allowed {
					rateLimitRejected.WithLabelValues("redis").Inc()
					return deny429(c, identifier)
				}
				rateLimitAllowed.WithLabelValues("redis").Inc()
				return next(c)
			}

			// Memory backend (no Redis configured): L1 only.
			rateLimitAllowed.WithLabelValues("l1").Inc()
			return next(c)
		}
	}
}
