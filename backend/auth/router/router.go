package router

import (
	"github.com/Sephy314/chinwag/auth/handler"
	"github.com/Sephy314/chinwag/auth/mocked"
	"github.com/Sephy314/chinwag/auth/repo"
	"github.com/Sephy314/chinwag/auth/service"
	"github.com/Sephy314/chinwag/conn"
	"github.com/Sephy314/chinwag/conn/cache"
	"github.com/labstack/echo/v5"
)

func SetUpAuthRouter(e *echo.Echo) {

	conns, err := conn.NewConnection()

	if err != nil {
		panic(err)
	}

	cacheRedis := cache.NewRedisCache(conns.Rds)

	userRepo := repo.NewUserRepository(conns)
	jwksRepo := repo.NewJwtRepository(conns)

	userService := service.NewUserService(cacheRedis, userRepo, &mocked.JwkService{}, &mocked.RefreshTokenService{})
	jwksService := service.NewJwksService(jwksRepo)

	userHandler := handler.NewUserHandler(userService)
	jwksHandler := handler.NewJwksHandler(jwksService)

	auth := e.Group("/auth")
	{
		auth.GET("/health", userHandler.Health)

		auth.POST("/user", userHandler.CreateUser)

		auth.POST("/login", userHandler.Login)

		auth.GET("/.well-known/jwks.json", jwksHandler.ServeJWKS)

	}

}
