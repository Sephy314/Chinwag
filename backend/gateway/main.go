package main

import (
	"log/slog"
	"net/http"
	"os"

	appMiddleware "github.com/Sephy314/chinwag/backend/gateway/middleware"
	"github.com/joho/godotenv"
	"github.com/labstack/echo/v5"
	"github.com/labstack/echo/v5/middleware"
)

func main() {
	_ = godotenv.Load()

	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))

	cfg := LoadConfig()

	e := echo.New()

	// Resolve the real client IP through Traefik's X-Forwarded-For so the rate
	// limiter keys on the actual client, not the Traefik pod. Traefik always
	// appends the connecting client IP as the rightmost XFF entry; the default
	// trusts private/cluster networks as the proxy hop.
	e.IPExtractor = echo.ExtractIPFromXFFHeader()

	// Distributed rate limiting: Redis is the authoritative global state
	// (shared across the 2 gateway replicas), with a per-pod L1 in-memory
	// limiter as the fast path and Redis-outage fallback. In "memory" backend
	// (local dev without Redis) only L1 runs.
	var redisLimiter appMiddleware.Limiter
	if cfg.RateLimitBackend == "redis" {
		redisLimiter = appMiddleware.NewRedisLimiter(
			cfg.RedisAddr, cfg.RedisPassword,
			cfg.RateLimitRate, cfg.RateLimitBurst,
			cfg.RedisKeyTTL, cfg.RedisTimeout,
		)
		slog.Info("rate limiter backend", "backend", "redis",
			"addr", cfg.RedisAddr, "rate", cfg.RateLimitRate, "burst", cfg.RateLimitBurst,
			"ttl", cfg.RedisKeyTTL.String(), "timeout", cfg.RedisTimeout.String())
	} else {
		slog.Info("rate limiter backend", "backend", "memory (L1 only)",
			"rate", cfg.RateLimitRate, "burst", cfg.RateLimitBurst)
	}

	e.Use(appMiddleware.RequestID())
	e.Use(appMiddleware.AccessLogger())
	e.Use(appMiddleware.MetricsMiddleware())

	e.Pre(middleware.CORSWithConfig(middleware.CORSConfig{
		AllowOrigins: []string{"http://localhost:3000"},
		AllowHeaders: []string{
			echo.HeaderOrigin,
			echo.HeaderContentType,
			echo.HeaderAccept,
			echo.HeaderAuthorization,
			echo.HeaderXRequestID,
			"DPoP",
		},
		ExposeHeaders: []string{
			"DPoP-Nonce",
		},
		AllowMethods: []string{
			http.MethodGet,
			http.MethodPost,
			http.MethodPut,
			http.MethodDelete,
			http.MethodOptions,
		},
		AllowCredentials: true,
	}))

	// Rate limit as a Pre middleware registered BEFORE setupRoutes. The proxy
	// router lives in a Pre middleware (see proxy.go) and short-circuits — it
	// proxies the request and returns, bypassing the Use chain. So the limiter
	// must run earlier in the Pre chain to actually throttle the proxied API
	// routes (/auth, /rooms, /chat, /users, /admin), not just /health.
	e.Pre(appMiddleware.RateLimit(appMiddleware.RateLimitConfig{
		Enabled:      cfg.RateLimitEnabled,
		Rate:         cfg.RateLimitRate,
		Burst:        cfg.RateLimitBurst,
		Redis:        redisLimiter,
		RedisTimeout: cfg.RedisTimeout,
		L1ExpiresIn:  cfg.L1ExpiresIn,
	}))

	e.GET("/health", func(c *echo.Context) error {
		return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
	})

	// Prometheus scrape endpoint (cluster-internal only; not exposed via the
	// public ingress). Scraped by the chart's kubernetes-service-endpoints job
	// through the annotations on infra/k3s/gateway.yaml.
	e.GET("/metrics", appMiddleware.MetricsHandler())

	setupRoutes(e, cfg)

	slog.Info("gateway starting",
		"port", cfg.Port,
		"routes", len(cfg.Routes),
		"rate_limit_enabled", cfg.RateLimitEnabled,
		"rate_limit_rate", cfg.RateLimitRate,
		"rate_limit_burst", cfg.RateLimitBurst,
	)

	if err := e.Start("0.0.0.0:" + cfg.Port); err != nil {
		slog.Error("gateway failed to start", "error", err)
		os.Exit(1)
	}
}
