package router

import (
	"context"
	"time"

	"github.com/Sephy314/chinwag/backend/monolith/auth/handler"
	"github.com/Sephy314/chinwag/backend/monolith/auth/oauth"
	"github.com/Sephy314/chinwag/backend/monolith/auth/repo"
	"github.com/Sephy314/chinwag/backend/monolith/auth/scheduler"
	"github.com/Sephy314/chinwag/backend/monolith/auth/service"
	"github.com/Sephy314/chinwag/backend/monolith/conn"
	"github.com/Sephy314/chinwag/backend/monolith/conn/bridge"
	"github.com/Sephy314/chinwag/backend/monolith/conn/cache"
	appMiddleware "github.com/Sephy314/chinwag/backend/monolith/middleware"
	"github.com/Sephy314/chinwag/backend/monolith/shared/keyProvider"
	"github.com/Sephy314/chinwag/backend/monolith/shared/logger"
	echojwt "github.com/labstack/echo-jwt/v5"
	"github.com/labstack/echo/v5"
)

func SetUpAuthRouter(e *echo.Echo, roomMember bridge.RoomMemberProvider, jwksService service.JwksServiceInterface, log logger.Logger) *service.UserService {

	conns, err := conn.NewConnection()

	if err != nil {
		panic(err)
	}

	cacheRedis := cache.NewRedisCache(conns.Rds)

	userRepo := repo.NewUserRepository(conns.DB)
	unitOfWork := repo.NewSQLUnitOfWork(conns.DB)

	refreshTokenService := service.NewRefreshTokenService(cacheRedis, "refresh:", time.Hour*24*14)

	keyRotationScheduler := scheduler.NewKeyRotationScheduler(jwksService, scheduler.NextMidnight(), log)
	go keyRotationScheduler.Start(context.Background())

	userService := service.NewUserService(cacheRedis, userRepo, jwksService, refreshTokenService, roomMember, log, unitOfWork)
	jwtService := service.NewJwtService(refreshTokenService, jwksService)

	refreshTokenHandler := handler.NewRefreshHandler(refreshTokenService, jwtService)

	userHandler := handler.NewUserHandler(userService, log)
	jwksHandler := handler.NewJwksHandler(jwksService)

	authPub := e.Group("/auth")
	{
		// Stricter rate limit for auth endpoints: 20 requests per minute per IP
		authRateLimitStore := appMiddleware.NewRedisSlidingWindowStore(conns.Rds, 20, time.Minute)
		authRateLimit := appMiddleware.NewRateLimitMiddleware(authRateLimitStore, appMiddleware.IPExtractor)

		authPub.GET("/health", userHandler.Health)

		authPub.GET("/user/id/:id", userHandler.GetUserByID)
		authPub.GET("/user/email/:email", userHandler.GetUserByEmail)
		authPub.POST("/user", userHandler.CreateUser, authRateLimit)

		authPub.POST("/login", userHandler.Login, authRateLimit)
		authPub.POST("/logout", userHandler.Logout)

		authPub.GET("/.well-known/jwks.json", jwksHandler.ServeJWKS)

		authPub.POST("/refresh", refreshTokenHandler.Refresh, authRateLimit)

		googleCfg := oauth.LoadGoogleConfig()
		if googleCfg.IsValid() {
			frontendURL := "http://localhost:3000"
			googleOAuthHandler := oauth.NewGoogleOAuthHandler(googleCfg, userService, jwksService, refreshTokenService, frontendURL, log)
			authPub.GET("/google", googleOAuthHandler.HandleRedirect)
			authPub.GET("/google/callback", googleOAuthHandler.HandleCallback)
		} else {
			log.Warn("google oauth not configured (missing GOOGLE_CLIENT_ID, GOOGLE_CLIENT_SECRET, or GOOGLE_REDIRECT_URL)")
		}
	}

	authPriv := e.Group("/auth")

	authPriv.Use(echojwt.WithConfig(echojwt.Config{
		KeyFunc: keyProvider.KeyFunc,
		ErrorHandler: func(c *echo.Context, err error) error {
			log.Error("jwt error", "error", err)
			return echo.ErrUnauthorized
		},
	}))

	{
		authPriv.GET("/whoami", userHandler.WhoAmI)
		authPriv.PUT("/user/:id", userHandler.UpdateUser)
		authPriv.DELETE("/user/:id", userHandler.DeleteUser)
	}

	log.Info("auth routes registered")

	return userService
}
