package router

import (
	"github.com/Sephy314/chinwag/auth/router"
	"github.com/Sephy314/chinwag/conn"
	"github.com/labstack/echo/v5"
	"github.com/labstack/echo/v5/middleware"
)

func SetUpRouter() (*echo.Echo, error) {
	_, err := conn.NewConnection()
	if err != nil {
		return nil, err
	}

	e := echo.New()

	e.Use(middleware.RequestID())
	e.Use(middleware.RequestLogger())

	e.Use(middleware.Recover())
	e.Use(middleware.CORSWithConfig(middleware.CORSConfig{
		AllowOrigins: []string{"*"},
	}))

	router.SetUpAuthRouter(e)

	return e, nil
}
