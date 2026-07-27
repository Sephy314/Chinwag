package router

import (
	"context"
	"errors"
	"net/http"

	authRepo "github.com/Sephy314/chinwag/auth/repo"
	authRouter "github.com/Sephy314/chinwag/auth/router"
	authService "github.com/Sephy314/chinwag/auth/service"
	chatRouter "github.com/Sephy314/chinwag/chat/router"
	"github.com/Sephy314/chinwag/conn"
	"github.com/Sephy314/chinwag/conn/bridge"
	appMiddleware "github.com/Sephy314/chinwag/middleware"
	roomRouter "github.com/Sephy314/chinwag/room/router"
	"github.com/Sephy314/chinwag/shared/keyProvider"
	"github.com/Sephy314/chinwag/shared/logger"
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

func SetUpRouter(log logger.Logger) (*echo.Echo, error) {
	conns, err := conn.NewConnection()
	if err != nil {
		return nil, err
	}

	jwksRepo := authRepo.NewJwtRepository(conns.DB)
	jwksService := authService.NewJwksService(jwksRepo, log)
	keyProvider.InjectProvider(jwksService)
	log.Info("key provider injected")

	e := echo.New()

	if e == nil {
		return nil, errors.New("no echo object")
	}

	e.HTTPErrorHandler = appMiddleware.GlobalErrorHandler(log)

	e.Use(middleware.RequestID())
	e.Use(appMiddleware.RequestIDInjector())
	e.Use(appMiddleware.ResponseIDInjector())
	e.Use(middleware.RequestLogger())
	//
	//rateLimiterStore := appMiddleware.NewRedisRateLimiterStore(conns.Rds, 50, 10, time.Minute)
	//e.Use(middleware.RateLimiterWithConfig(
	//	middleware.RateLimiterConfig{
	//		Store: rateLimiterStore,
	//		IdentifierExtractor: func(c *echo.Context) (string, error) {
	//			return c.RealIP(), nil
	//		},
	//	},
	//))

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

	roomMemberProv := roomRouter.SetUpRoomRouter(e, userAdapter, log)

	chatRouter.SetUpChatRouter(e, userAdapter, roomMemberProv, log)

	userService := authRouter.SetUpAuthRouter(e, roomMemberProv, jwksService, log)
	userAdapter.SetUserService(&userServiceAdapter{svc: userService})

	return e, nil
}
