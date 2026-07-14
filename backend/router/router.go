package router

import (
	"errors"
	"io/fs"
	"net/http"

	authRouter "github.com/Sephy314/chinwag/auth/router"
	"github.com/Sephy314/chinwag/conn"
	"github.com/Sephy314/chinwag/docs"
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

	// TODO: Restrict AllowOrigins to trusted domains before production deployment.
	// Wildcard "*" allows any origin, which is insecure for authenticated endpoints.
	e.Use(middleware.CORSWithConfig(middleware.CORSConfig{
		AllowOrigins: []string{"*"},
	}))

	e.GET("/swagger/swagger.json", func(c *echo.Context) error {
		return c.JSONBlob(http.StatusOK, docs.SwaggerJSON)
	})

	staticFS, _ := fs.Sub(docs.StaticFS, "swagger-ui")
	fileServer := http.StripPrefix("/swagger/swagger-ui/", http.FileServer(http.FS(staticFS)))

	e.GET("/swagger/swagger-ui/*", func(c *echo.Context) error {
		path := c.Request().URL.Path
		if path == "/swagger/swagger-ui/" || path == "/swagger/swagger-ui" {
			http.Redirect(c.Response(), c.Request(), "/swagger", http.StatusMovedPermanently)
			return nil
		}
		fileServer.ServeHTTP(c.Response(), c.Request())
		return nil
	})

	e.GET("/swagger", func(c *echo.Context) error {
		return c.HTML(http.StatusOK, string(docs.IndexHTML))
	})

	authRouter.SetUpAuthRouter(e)
	roomRouter.SetUpRoomRouter(e)

	return e, nil
}
