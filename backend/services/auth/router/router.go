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

func (r *Router) requestLogger() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) error {
			start := time.Now()
			err := next(c)
			rid := c.Response().Header().Get(echo.HeaderXRequestID)
			status := 0
			if resp, rErr := echo.UnwrapResponse(c.Response()); rErr == nil {
				status = resp.Status
			}
			r.log.Info("request",
				"request_id", rid,
				"method", c.Request().Method,
				"path", c.Request().URL.Path,
				"status", status,
				"latency", time.Since(start).String(),
			)
			return err
		}
	}
}

func (r *Router) Setup(cfg *RouterConfig) {
	e := r.Echo

	e.Use(middleware.RequestID())
	e.Use(middleware.Recover())
	e.Use(r.requestLogger())

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

	jwksClient := sharedauth.NewJWKSClient("http://localhost:"+cfg.Port+"/.well-known/jwks.json", time.Minute*10)

	pub := e.Group("")
	{
		pub.GET("/health", func(c *echo.Context) error {
			return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
		})

		pub.GET("/.well-known/jwks.json", r.JwksHandler.ServeJWKS)
		pub.POST("/login", r.UserHandler.Login)
		pub.POST("/refresh", r.RefreshHandler.Refresh)
		pub.POST("/logout", r.UserHandler.Logout)
		pub.POST("/user", r.UserHandler.CreateUser)

		if cfg.GoogleOAuthEnabled {
			googleOAuthHandler := oauth.NewGoogleOAuthHandler(
				cfg.GoogleConfig,
				r.UserHandler.Service,
				r.JwksService,
				r.UserHandler.Service.RefreshService,
				cfg.FrontendURL,
				r.log,
			)
			pub.GET("/google", googleOAuthHandler.HandleRedirect)
			pub.GET("/google/callback", googleOAuthHandler.HandleCallback)
		} else {
			r.log.Warn("google oauth not configured")
		}
	}

	priv := e.Group("")
	priv.Use(sharedauth.NewMiddleware(jwksClient))
	{
		priv.GET("/whoami", r.UserHandler.WhoAmI)
		priv.GET("/user/:id", r.UserHandler.GetUserByID)
		priv.GET("/user/email/:email", r.UserHandler.GetUserByEmail)
		priv.PUT("/user/:id", r.UserHandler.UpdateUser)
		priv.DELETE("/user/:id", r.UserHandler.DeleteUser)
	}

	SetUpSwaggerRoutes(e)
}

type RouterConfig struct {
	Port               string
	FrontendURL        string
	GoogleOAuthEnabled bool
	GoogleConfig       *oauth.GoogleConfig
}
