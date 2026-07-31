package router

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/Sephy314/chinwag/backend/services/chat/query/handler"
	sharedauth "github.com/Sephy314/chinwag/backend/shared/auth"
	"github.com/labstack/echo/v5"
	"github.com/labstack/echo/v5/middleware"
)

type Router struct {
	Echo         *echo.Echo
	QueryHandler *handler.QueryHandler
	log          *slog.Logger
}

func NewRouter(
	queryHandler *handler.QueryHandler,
	log *slog.Logger,
) *Router {
	return &Router{
		Echo:         echo.New(),
		QueryHandler: queryHandler,
		log:          log,
	}
}

func (r *Router) Setup(cfg *RouterConfig) {
	e := r.Echo

	e.Use(middleware.RequestID())
	e.Use(middleware.Recover())
	e.Use(middleware.RequestLogger())

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
	priv.Use(sharedauth.NewMiddleware(jwksClient))

	{
		priv.GET("/chat/rooms/:roomId/messages", r.QueryHandler.ListMessages)
		priv.GET("/chat/rooms/:roomId/messages/:messageId", r.QueryHandler.GetMessage)
	}

	r.log.Info("chat query routes registered")
}

type RouterConfig struct {
	Port        string
	JWKSURL     string
	FrontendURL string
}
