package router

import (
	"net/http"
	"time"

	"github.com/Sephy314/chinwag/backend/services/auth/handler"
	"github.com/Sephy314/chinwag/backend/services/auth/oauth"
	"github.com/Sephy314/chinwag/backend/services/auth/service"
	"github.com/Sephy314/chinwag/backend/services/auth/shared/logger"
	sharedauth "github.com/Sephy314/chinwag/backend/shared/auth"
	"github.com/labstack/echo/v5"
	"github.com/labstack/echo/v5/middleware"
)

type Router struct {
	Echo           *echo.Echo
	UserHandler    *handler.UserHandler
	JwksHandler    *handler.JwksHandler
	RefreshHandler *handler.RefreshHandlerImpl
	JwksService    *service.JwksService
	log            logger.Logger
}

func NewRouter(
	userHandler *handler.UserHandler,
	jwksHandler *handler.JwksHandler,
	refreshHandler *handler.RefreshHandlerImpl,
	jwksService *service.JwksService,
	log logger.Logger,
) *Router {
	return &Router{
		Echo:           echo.New(),
		UserHandler:    userHandler,
		JwksHandler:    jwksHandler,
		RefreshHandler: refreshHandler,
		JwksService:    jwksService,
		log:            log,
	}
}

func (r *Router) Setup(cfg *RouterConfig) {
	e := r.Echo

	e.Use(middleware.RequestID())
	e.Use(middleware.Recover())

	e.Use(middleware.CORSWithConfig(middleware.CORSConfig{
		AllowOrigins: []string{"http://localhost:3000"},
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

	e.GET("/health", func(c *echo.Context) error {
		return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
	})

	SetUpSwaggerRoutes(e)

	e.GET("/.well-known/jwks.json", r.JwksHandler.ServeJWKS)
	e.POST("/login", r.UserHandler.Login)
	e.POST("/refresh", r.RefreshHandler.Refresh)
	e.POST("/logout", r.UserHandler.Logout)
	e.POST("/user", r.UserHandler.CreateUser)

	jwksClient := sharedauth.NewJWKSClient("http://localhost:"+cfg.Port+"/.well-known/jwks.json", time.Minute*10)
	auth := sharedauth.NewMiddleware(jwksClient)

	e.GET("/user/me", r.UserHandler.WhoAmI, auth)
	e.GET("/user/:id", r.UserHandler.GetUserByID, auth)
	e.GET("/user/email/:email", r.UserHandler.GetUserByEmail, auth)
	e.PUT("/user/:id", r.UserHandler.UpdateUser, auth)
	e.DELETE("/user/:id", r.UserHandler.DeleteUser, auth)

	if cfg.GoogleOAuthEnabled {
		googleOAuthHandler := oauth.NewGoogleOAuthHandler(
			cfg.GoogleConfig,
			r.UserHandler.Service,
			r.JwksService,
			r.UserHandler.Service.RefreshService,
			cfg.FrontendURL,
			r.log,
		)
		e.GET("/google", googleOAuthHandler.HandleRedirect)
		e.GET("/google/callback", googleOAuthHandler.HandleCallback)
	} else {
		r.log.Warn("google oauth not configured")
	}
}

type RouterConfig struct {
	Port               string
	FrontendURL        string
	GoogleOAuthEnabled bool
	GoogleConfig       *oauth.GoogleConfig
}
