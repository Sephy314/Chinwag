package router

import (
	authRouter "github.com/Sephy314/chinwag/auth/router"
	"github.com/Sephy314/chinwag/conn"
	roomRouter "github.com/Sephy314/chinwag/room/router"
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

	authRouter.SetUpAuthRouter(e)
	roomRouter.SetUpRoomRouter(e)

	return e, nil
}
