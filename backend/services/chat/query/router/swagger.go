package router

import (
	"github.com/labstack/echo/v5"
	"github.com/labstack/echo/v5/middleware"
)

func SetUpSwaggerRoutes(e *echo.Echo) {
	e.Use(middleware.StaticWithConfig(middleware.StaticConfig{
		Root:       "docs",
		Skipper:    func(c *echo.Context) bool { return true },
	}))
}
