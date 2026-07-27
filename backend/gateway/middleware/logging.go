package middleware

import (
	"log/slog"
	"time"

	"github.com/labstack/echo/v5"
)

func AccessLogger() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) error {
			start := time.Now()

			err := next(c)

			rid, _ := echo.ContextGet[string](c, ContextKeyRequestID)

			status := 0
			if r, rErr := echo.UnwrapResponse(c.Response()); rErr == nil {
				status = r.Status
			}

			slog.Info("request",
				"request_id", rid,
				"timestamp", start.Format(time.RFC3339),
				"client_ip", c.RealIP(),
				"method", c.Request().Method,
				"path", c.Request().URL.Path,
				"status", status,
				"latency", time.Since(start).String(),
			)

			return err
		}
	}
}
