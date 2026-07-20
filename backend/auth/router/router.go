package router

import (
	"log"
	"time"

	"github.com/Sephy314/chinwag/auth/handler"
	"github.com/Sephy314/chinwag/auth/repo"
	"github.com/Sephy314/chinwag/auth/service"
	"github.com/Sephy314/chinwag/conn"
	"github.com/Sephy314/chinwag/conn/bridge"
	"github.com/Sephy314/chinwag/conn/cache"
	"github.com/Sephy314/chinwag/shared/keyProvider"
	echojwt "github.com/labstack/echo-jwt/v5"
	"github.com/labstack/echo/v5"
)

func SetUpAuthRouter(e *echo.Echo, roomMember bridge.RoomMemberProvider) *service.UserService {

	conns, err := conn.NewConnection()

	if err != nil {
		panic(err)
	}

	cacheRedis := cache.NewRedisCache(conns.Rds)

	userRepo := repo.NewUserRepository(conns.DB)
	jwksRepo := repo.NewJwtRepository(conns.DB)
	unitOfWork := repo.NewSQLUnitOfWork(conns.DB)

	refreshTokenService := service.NewRefreshTokenService(cacheRedis, "refresh:", time.Minute*5)

	jwksService := service.NewJwksService(jwksRepo)
	userService := service.NewUserService(cacheRedis, userRepo, jwksService, refreshTokenService, roomMember, unitOfWork)
	jwtService := service.NewJwtService(refreshTokenService, jwksService)

	refreshTokenHandler := handler.NewRefreshHandler(refreshTokenService, jwtService)

	keyProvider.InjectProvider(jwksService)
	log.Println("Key Provider Injected")

	userHandler := handler.NewUserHandler(userService)
	jwksHandler := handler.NewJwksHandler(jwksService)

	authPub := e.Group("/auth")
	{
		authPub.GET("/health", userHandler.Health)

		authPub.GET("/user/:id", userHandler.GetUser)
		authPub.POST("/user", userHandler.CreateUser)

		authPub.POST("/login", userHandler.Login)

		authPub.GET("/.well-known/jwks.json", jwksHandler.ServeJWKS)

		authPub.POST("/refresh", refreshTokenHandler.Refresh)

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
	}

	log.Println("auth routes registered")

	return userService
}
