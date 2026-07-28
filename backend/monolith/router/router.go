package router

import (
	"errors"
	"net/http"
	"time"

	"github.com/Sephy314/chinwag/backend/monolith/conn"
	appMiddleware "github.com/Sephy314/chinwag/backend/monolith/middleware"
	"github.com/Sephy314/chinwag/backend/monolith/shared/logger"
	"github.com/labstack/echo/v5"
	"github.com/labstack/echo/v5/middleware"
)

func SetUpRouter(log logger.Logger) (*echo.Echo, error) {
	conns, err := conn.NewConnection()
	if err != nil {
		return nil, err
	}

	e := echo.New()

	if e == nil {
		return nil, errors.New("no echo object")
	}

	e.HTTPErrorHandler = appMiddleware.GlobalErrorHandler(log)

	e.Use(middleware.RequestID())
	e.Use(appMiddleware.RequestIDInjector())
	e.Use(appMiddleware.ResponseIDInjector())
	e.Use(middleware.RequestLogger())

	// Global rate limiter: 100 requests per minute per IP
	globalStore := appMiddleware.NewRedisSlidingWindowStore(conns.Rds, 100, time.Minute)
	e.Use(appMiddleware.NewRateLimitMiddleware(globalStore, appMiddleware.IPExtractor))

	e.Use(middleware.Recover())

	e.Use(middleware.CORSWithConfig(middleware.CORSConfig{
		AllowOrigins: []string{
			"http://localhost:3000",
		},
		AllowHeaders: []string{
			echo.HeaderOrigin,
			echo.HeaderContentType,
			echo.HeaderAccept,
			echo.HeaderAuthorization,
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

	SetUpSwaggerRoutes(e)

	return e, nil
}
