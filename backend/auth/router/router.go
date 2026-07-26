package router

import (
	"context"
	"log"
	"time"

	"github.com/Sephy314/chinwag/auth/handler"
	"github.com/Sephy314/chinwag/auth/oauth"
	"github.com/Sephy314/chinwag/auth/repo"
	"github.com/Sephy314/chinwag/auth/scheduler"
	"github.com/Sephy314/chinwag/auth/service"
	"github.com/Sephy314/chinwag/conn"
	"github.com/Sephy314/chinwag/conn/bridge"
	"github.com/Sephy314/chinwag/conn/cache"
	"github.com/Sephy314/chinwag/shared/keyProvider"
	echojwt "github.com/labstack/echo-jwt/v5"
	"github.com/labstack/echo/v5"
)

func SetUpAuthRouter(e *echo.Echo, roomMember bridge.RoomMemberProvider, jwksService service.JwksServiceInterface) *service.UserService {

	conns, err := conn.NewConnection()

	if err != nil {
		panic(err)
	}

	cacheRedis := cache.NewRedisCache(conns.Rds)

	userRepo := repo.NewUserRepository(conns.DB)
	unitOfWork := repo.NewSQLUnitOfWork(conns.DB)

	refreshTokenService := service.NewRefreshTokenService(cacheRedis, "refresh:", time.Hour*24*14)

	keyRotationScheduler := scheduler.NewKeyRotationScheduler(jwksService, scheduler.NextMidnight())
	go keyRotationScheduler.Start(context.Background())

	userService := service.NewUserService(cacheRedis, userRepo, jwksService, refreshTokenService, roomMember, unitOfWork)
	jwtService := service.NewJwtService(refreshTokenService, jwksService)

	refreshTokenHandler := handler.NewRefreshHandler(refreshTokenService, jwtService)

	userHandler := handler.NewUserHandler(userService)
	jwksHandler := handler.NewJwksHandler(jwksService)

	authPub := e.Group("/auth")
	{
		authPub.GET("/health", userHandler.Health)

		authPub.GET("/user/id/:id", userHandler.GetUserByID)
		authPub.GET("/user/email/:email", userHandler.GetUserByEmail)
		authPub.POST("/user", userHandler.CreateUser)

		authPub.POST("/login", userHandler.Login)

		authPub.GET("/.well-known/jwks.json", jwksHandler.ServeJWKS)

		authPub.POST("/refresh", refreshTokenHandler.Refresh)

		googleCfg := oauth.LoadGoogleConfig()
		if googleCfg.IsValid() {
			frontendURL := "http://localhost:3000"
			googleOAuthHandler := oauth.NewGoogleOAuthHandler(googleCfg, userService, jwksService, refreshTokenService, frontendURL)
			authPub.GET("/google", googleOAuthHandler.HandleRedirect)
			authPub.GET("/google/callback", googleOAuthHandler.HandleCallback)
		} else {
			log.Println("Google OAuth not configured (missing GOOGLE_CLIENT_ID, GOOGLE_CLIENT_SECRET, or GOOGLE_REDIRECT_URL)")
		}
	}

	authPriv := e.Group("/auth")

	authPriv.Use(echojwt.WithConfig(echojwt.Config{
		KeyFunc: keyProvider.KeyFunc,
		ErrorHandler: func(c *echo.Context, err error) error {
			log.Println(err)
			return echo.ErrUnauthorized
		},
	}))

	{
		authPriv.GET("/whoami", userHandler.WhoAmI)
		authPriv.PUT("/user/:id", userHandler.UpdateUser)
		authPriv.DELETE("/user/:id", userHandler.DeleteUser)
	}

	log.Println("auth routes registered")

	return userService
}
