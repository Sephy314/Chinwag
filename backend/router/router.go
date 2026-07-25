package router

import (
	"context"
	"errors"
	"log"
	"net/http"

	authRepo "github.com/Sephy314/chinwag/auth/repo"
	authRouter "github.com/Sephy314/chinwag/auth/router"
	authService "github.com/Sephy314/chinwag/auth/service"
	chatRouter "github.com/Sephy314/chinwag/chat/router"
	"github.com/Sephy314/chinwag/conn"
	"github.com/Sephy314/chinwag/conn/bridge"
	appMiddleware "github.com/Sephy314/chinwag/middleware"
	roomRouter "github.com/Sephy314/chinwag/room/router"
	"github.com/Sephy314/chinwag/shared/errs"
	"github.com/Sephy314/chinwag/shared/keyProvider"
	"github.com/Sephy314/chinwag/shared/response"
	"github.com/labstack/echo/v5"
	"github.com/labstack/echo/v5/middleware"
)

type userServiceAdapter struct {
	svc *authService.UserService
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
	conns, err := conn.NewConnection()
	if err != nil {
		return nil, err
	}

	jwksRepo := authRepo.NewJwtRepository(conns.DB)
	jwksService := authService.NewJwksService(jwksRepo)
	keyProvider.InjectProvider(jwksService)
	log.Println("Key Provider Injected")

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

	userAdapter := bridge.NewUserAdapter(func(ctx context.Context, id string) (*bridge.UserInfo, error) {
		return nil, nil
	})

	roomMemberProv := roomRouter.SetUpRoomRouter(e, userAdapter)

	chatRouter.SetUpChatRouter(e, userAdapter, roomMemberProv)

	userService := authRouter.SetUpAuthRouter(e, roomMemberProv, jwksService)
	userAdapter.SetUserService(&userServiceAdapter{svc: userService})

	return e, nil
}
