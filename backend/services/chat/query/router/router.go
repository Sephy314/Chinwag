package router

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/Sephy314/chinwag/backend/services/chat/query/handler"
	sharedauth "github.com/Sephy314/chinwag/backend/shared/auth"
	"github.com/Sephy314/chinwag/backend/shared/auth/dpop"
	"github.com/labstack/echo/v5"
	"github.com/labstack/echo/v5/middleware"
)

type Router struct {
	Echo              *echo.Echo
	QueryHandler      *handler.QueryHandler
	AdminQueryHandler *handler.AdminQueryHandler
	log               *slog.Logger
}

func NewRouter(
	queryHandler *handler.QueryHandler,
	adminQueryHandler *handler.AdminQueryHandler,
	log *slog.Logger,
) *Router {
	return &Router{
		Echo:              echo.New(),
		QueryHandler:      queryHandler,
		AdminQueryHandler: adminQueryHandler,
		log:               log,
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
			http.MethodOptions,
		},
		AllowCredentials: true,
	}))

	jwksClient := sharedauth.NewJWKSClient(cfg.JWKSURL, time.Minute*10)

	pub := e.Group("")
	{
		pub.GET("/chat/health", r.QueryHandler.Health)
	}

	priv := e.Group("")
	priv.Use(sharedauth.NewMiddleware(jwksClient, r.log, cfg.DPoPValidator))

	{
		priv.GET("/chat/rooms/:roomId/messages", r.QueryHandler.ListMessages)
		priv.GET("/chat/rooms/:roomId/messages/:messageId", r.QueryHandler.GetMessage)
	}

	admin := e.Group("")
	admin.Use(sharedauth.NewMiddleware(jwksClient, r.log, cfg.DPoPValidator))
	admin.Use(sharedauth.RequireRole(sharedauth.RoleAdmin))
	{
		admin.GET("/chat/admin/messages", r.AdminQueryHandler.ListMessages)
		admin.GET("/chat/admin/messages/:messageId", r.AdminQueryHandler.GetMessage)
		admin.GET("/chat/admin/stats/messages", r.AdminQueryHandler.StatsMessages)
	}

	r.log.Info("chat query routes registered")
}

type RouterConfig struct {
	Port          string
	JWKSURL       string
	FrontendURL   string
	DPoPValidator *dpop.Validator
}
