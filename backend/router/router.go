package router

import (
	"errors"
	"net/http"

	authRouter "github.com/Sephy314/chinwag/auth/router"
	"github.com/Sephy314/chinwag/conn"
	roomRouter "github.com/Sephy314/chinwag/room/router"
	"github.com/Sephy314/chinwag/shared/errs"
	"github.com/labstack/echo/v5"
	"github.com/labstack/echo/v5/middleware"
)

func SetUpRouter() (*echo.Echo, error) {
	_, err := conn.NewConnection()
	if err != nil {
		return nil, err
	}

	e := echo.New()

	if e == nil {
		return nil, errors.New("no echo object")
	}

	e.HTTPErrorHandler = func(c *echo.Context, err error) {
		if appErr, ok := errors.AsType[*errs.AppError](err); ok {
			_ = c.JSON(appErr.Status, map[string]any{
				"success": false,
				"message": appErr.Message,
			})
			return
		}

		_ = c.JSON(http.StatusInternalServerError, map[string]any{
			"success": false,
			"message": "internal server error",
		})
	}

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
