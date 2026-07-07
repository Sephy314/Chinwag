package router

import (
	"context"
	"crypto/ecdsa"
	"fmt"
	"time"

	"github.com/Sephy314/chinwag/auth/handler"
	"github.com/Sephy314/chinwag/auth/repo"
	"github.com/Sephy314/chinwag/auth/service"
	"github.com/Sephy314/chinwag/conn"
	"github.com/Sephy314/chinwag/conn/cache"
	echojwt "github.com/labstack/echo-jwt/v5"
	"github.com/labstack/echo/v5"
	gjwt "github.com/golang-jwt/jwt/v4"
	"github.com/lestrrat-go/jwx/v3/jwk"
)

func SetUpAuthRouter(e *echo.Echo) {

	conns, err := conn.NewConnection()

	if err != nil {
		panic(err)
	}

	cacheRedis := cache.NewRedisCache(conns.Rds)

	userRepo := repo.NewUserRepository(conns)
	jwksRepo := repo.NewJwtRepository(conns)

	refreshTokenService := service.NewRefreshTokenService(cacheRedis, "refresh:", time.Minute*5)

	jwksService := service.NewJwksService(jwksRepo)
	userService := service.NewUserService(cacheRedis, userRepo, jwksService, refreshTokenService)
	jwtService := service.NewJwtService(refreshTokenService, jwksService)

	refreshTokenHandler := handler.NewRefreshHandler(refreshTokenService, jwtService)

	userHandler := handler.NewUserHandler(userService)
	jwksHandler := handler.NewJwksHandler(jwksService)

	auth := e.Group("/auth")
	{
		auth.GET("/health", userHandler.Health)

		auth.GET("/user/:id", userHandler.GetUser)
		auth.POST("/user", userHandler.CreateUser)

		auth.POST("/login", userHandler.Login)

		auth.GET("/.well-known/jwks.json", jwksHandler.ServeJWKS)

		auth.POST("/refresh", refreshTokenHandler.Refresh)

	}

	auth.Use(echojwt.WithConfig(echojwt.Config{
		KeyFunc: func(token *gjwt.Token) (interface{}, error) {
			kidVal, ok := token.Header["kid"]
			if !ok {
				return nil, fmt.Errorf("missing kid header")
			}
			kid, ok := kidVal.(string)
			if !ok || kid == "" {
				return nil, fmt.Errorf("invalid kid header")
			}

			keys, err := jwksService.GetJwkSet(context.Background())
			if err != nil {
				return nil, err
			}

			for _, k := range keys.Keys() {
				v, err := k.Get(jwk.KeyIDKey)
				if err != nil {
					continue
				}
				if ks, ok := v.(string); ok && ks == kid {
					// try to materialize into ecdsa.PublicKey
					var pub ecdsa.PublicKey
					if err := k.Raw(&pub); err == nil {
						return &pub, nil
					}
					// fallback to Materialize
					raw, err := k.Materialize()
					if err == nil {
						if p, ok := raw.(*ecdsa.PublicKey); ok {
							return p, nil
						}
					}
				}
			}

			return nil, fmt.Errorf("public key for kid %s not found", kid)
		},
	}))

	//
	//policy := []middleware.PathAuthenticatedPolicy{
	//	{
	//		Path:   "health",
	//		Method: "GET",
	//		Role:   nil,
	//		Auth:   false,
	//	},
	//	{
	//		Path:   "user",
	//		Method: "GET",
	//		Role:   nil,
	//		Auth:   false,
	//	},
	//	{
	//		Path:   "login",
	//		Method: "GET",
	//		Role:   nil,
	//		Auth:   false,
	//	},
	//	{
	//		Path:   "refresh",
	//		Method: "GET",
	//		Role:   nil,
	//		Auth:   false,
	//	},
	//	{
	//		Path:   "user",
	//		Method: "GET",
	//	},
	//}
	//
	//auth.Use(middleware.AuthMiddleware(policy, jwksService))
}
