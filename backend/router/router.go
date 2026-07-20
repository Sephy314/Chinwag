package router

import (
	"context"
	"errors"
	"io/fs"
	"net/http"

	authRouter "github.com/Sephy314/chinwag/auth/router"
	"github.com/Sephy314/chinwag/auth/service"
	"github.com/Sephy314/chinwag/conn"
	"github.com/Sephy314/chinwag/conn/bridge"
	"github.com/Sephy314/chinwag/docs"
	appMiddleware "github.com/Sephy314/chinwag/middleware"
	roomRouter "github.com/Sephy314/chinwag/room/router"
	"github.com/Sephy314/chinwag/shared/errs"
	"github.com/Sephy314/chinwag/shared/response"
	"github.com/labstack/echo/v5"
	"github.com/labstack/echo/v5/middleware"
)

type userServiceAdapter struct {
	svc *service.UserService
}

func (a *userServiceAdapter) GetUser(ctx context.Context, id string) (*bridge.UserInfo, error) {
	user, err := a.svc.GetUser(ctx, id)
	if err != nil {
		return nil, err
	}
	return &bridge.UserInfo{
		Id:        user.Id,
		Name:      user.Name,
		Email:     user.Email,
		Role:      string(user.Role),
		CreatedAt: user.CreatedAt,
		UpdatedAt: user.UpdatedAt,
	}, nil
}

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
		if r, rErr := echo.UnwrapResponse(c.Response()); rErr == nil && r.Committed {
			return
		}

		code := http.StatusInternalServerError
		var msg string

		var sc echo.HTTPStatusCoder
		if errors.As(err, &sc) {
			if tmp := sc.StatusCode(); tmp != 0 {
				code = tmp
			}
		}

		if he, ok := errors.AsType[*echo.HTTPError](err); ok {
			msg = he.Message
			if msg == "" {
				msg = http.StatusText(code)
			}
		} else if appErr, ok := errors.AsType[*errs.AppError](err); ok {
			code = appErr.Status
			msg = appErr.Message
		} else {
			msg = http.StatusText(code)
		}

		_ = c.JSON(code, response.Error(msg))
	}

	e.Use(middleware.RequestID())
	e.Use(appMiddleware.RequestIDInjector())
	e.Use(appMiddleware.ResponseIDInjector())
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

	userAdapter := bridge.NewUserAdapter(func(ctx context.Context, id string) (*bridge.UserInfo, error) {
		return nil, nil
	})

	roomMemberProv := roomRouter.SetUpRoomRouter(e, userAdapter)

	userService := authRouter.SetUpAuthRouter(e, roomMemberProv)
	userAdapter.SetUserService(&userServiceAdapter{svc: userService})

	return e, nil
}
