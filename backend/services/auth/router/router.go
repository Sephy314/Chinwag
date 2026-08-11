package router

import (
	"net/http"
	"time"

	"github.com/Sephy314/chinwag/backend/services/auth/handler"
	"github.com/Sephy314/chinwag/backend/services/auth/oauth"
	"github.com/Sephy314/chinwag/backend/services/auth/service"
	"github.com/Sephy314/chinwag/backend/services/auth/shared/cache"
	"github.com/Sephy314/chinwag/backend/services/auth/shared/logger"
	sharedauth "github.com/Sephy314/chinwag/backend/shared/auth"
	"github.com/Sephy314/chinwag/backend/shared/auth/dpop"
	"github.com/labstack/echo/v5"
	"github.com/labstack/echo/v5/middleware"
)

type Router struct {
	Echo                *echo.Echo
	UserHandler         *handler.UserHandler
	JwksHandler         *handler.JwksHandler
	RefreshHandler      *handler.RefreshHandlerImpl
	JwksService         *service.JwksService
	AdminUserHandler    *handler.AdminUserHandler
	AdminSessionHandler *handler.AdminSessionHandler
	AdminAuditHandler   *handler.AdminAuditHandler
	log                 logger.Logger
}

func NewRouter(
	userHandler *handler.UserHandler,
	jwksHandler *handler.JwksHandler,
	refreshHandler *handler.RefreshHandlerImpl,
	jwksService *service.JwksService,
	adminUserHandler *handler.AdminUserHandler,
	adminSessionHandler *handler.AdminSessionHandler,
	adminAuditHandler *handler.AdminAuditHandler,
	log logger.Logger,
) *Router {
	return &Router{
		Echo:                echo.New(),
		UserHandler:         userHandler,
		JwksHandler:         jwksHandler,
		RefreshHandler:      refreshHandler,
		JwksService:         jwksService,
		AdminUserHandler:    adminUserHandler,
		AdminSessionHandler: adminSessionHandler,
		AdminAuditHandler:   adminAuditHandler,
		log:                 log,
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
			userID := ""
			if claims, cerr := sharedauth.ClaimsFromContext(c); cerr == nil {
				userID = claims.Subject
			}
			r.log.Info("request",
				"request_id", rid,
				"user_id", userID,
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
			"DPoP",
		},
		ExposeHeaders: []string{
			"DPoP-Nonce",
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
		pub.GET("/user/:id", r.UserHandler.GetUserByID)
		pub.GET("/user/email/:email", r.UserHandler.GetUserByEmail)

		// Internal, system-to-system audit write used by other services. Not
		// routed by the gateway, so it is not publicly reachable.
		pub.POST("/internal/audit", r.AdminAuditHandler.RecordAudit)

		if cfg.GoogleOAuthEnabled {
			googleOAuthHandler := oauth.NewGoogleOAuthHandler(
				cfg.GoogleConfig,
				r.UserHandler.Service,
				r.JwksService,
				r.UserHandler.Service.RefreshService,
				cfg.Cache,
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
	priv.Use(sharedauth.NewMiddleware(jwksClient, r.log, cfg.DPoPValidator))
	{
		priv.GET("/whoami", r.UserHandler.WhoAmI)
		priv.PUT("/user/:id", r.UserHandler.UpdateUser)
		priv.DELETE("/user/:id", r.UserHandler.DeleteUser)
	}

	admin := e.Group("")
	admin.Use(sharedauth.NewMiddleware(jwksClient, r.log, cfg.DPoPValidator))
	admin.Use(sharedauth.RequireRole(sharedauth.RoleAdmin))
	{
		admin.GET("/admin/users", r.AdminUserHandler.ListUsers)
		admin.POST("/admin/users", r.AdminUserHandler.CreateUser)
		admin.GET("/admin/users/:id", r.AdminUserHandler.GetUser)
		admin.PUT("/admin/users/:id", r.AdminUserHandler.UpdateUser)
		admin.PUT("/admin/users/:id/role", r.AdminUserHandler.UpdateRole)
		admin.DELETE("/admin/users/:id", r.AdminUserHandler.DisableUser)
		admin.POST("/admin/users/:id/restore", r.AdminUserHandler.RestoreUser)
		admin.GET("/admin/users/:id/sessions", r.AdminUserHandler.ListUserSessions)
		admin.DELETE("/admin/users/:id/sessions", r.AdminUserHandler.RevokeUserSessions)

		admin.GET("/admin/sessions", r.AdminSessionHandler.ListSessions)
		admin.GET("/admin/sessions/:id", r.AdminSessionHandler.GetSession)
		admin.DELETE("/admin/sessions/:id", r.AdminSessionHandler.RevokeSession)

		admin.GET("/admin/audit", r.AdminAuditHandler.ListAudit)

		admin.GET("/admin/stats/users", r.AdminUserHandler.StatsUsers)
		admin.GET("/admin/stats/sessions", r.AdminSessionHandler.StatsSessions)
	}

	SetUpSwaggerRoutes(e)
}

type RouterConfig struct {
	Port               string
	FrontendURL        string
	GoogleOAuthEnabled bool
	GoogleConfig       *oauth.GoogleConfig
	Cache              cache.Cache
	DPoPValidator      *dpop.Validator
}
