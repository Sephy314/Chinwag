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

	e.GET("/health", func(c *echo.Context) error {
		return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
	})

	// Prometheus scrape endpoint (cluster-internal only; not exposed via the
	// public ingress). Scraped by the chart's kubernetes-service-endpoints job
	// through the annotations on infra/k3s/gateway.yaml.
	e.GET("/metrics", appMiddleware.MetricsHandler())

	setupRoutes(e, cfg)

	slog.Info("gateway starting", "port", cfg.Port, "routes", len(cfg.Routes))

	if err := e.Start("0.0.0.0:" + cfg.Port); err != nil {
		slog.Error("gateway failed to start", "error", err)
		os.Exit(1)
	}
}
