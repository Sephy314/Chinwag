package router

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/Sephy314/chinwag/backend/services/chat/command/handler"
	sharedauth "github.com/Sephy314/chinwag/backend/shared/auth"
	"github.com/labstack/echo/v5"
	"github.com/labstack/echo/v5/middleware"
)

type Router struct {
	Echo        *echo.Echo
	ChatHandler *handler.ChatHandler
	WSHandler   *handler.WebSocketHandler
	log         *slog.Logger
}

func NewRouter(
	chatHandler *handler.ChatHandler,
	wsHandler *handler.WebSocketHandler,
	log *slog.Logger,
) *Router {
	return &Router{
		Echo:        echo.New(),
		ChatHandler: chatHandler,
		WSHandler:   wsHandler,
		log:         log,
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

	jwksClient := sharedauth.NewJWKSClient(cfg.JWKSURL, time.Minute*10)

	pub := e.Group("")
	{
		pub.GET("/chat/health", r.ChatHandler.Health)
		pub.GET("/chat/rooms/:roomId/ws", r.WSHandler.ServeWS)
	}

	priv := e.Group("")
	priv.Use(sharedauth.NewMiddleware(jwksClient, r.log, cfg.DPoPStore))
	{
		priv.POST("/chat/rooms/:roomId/messages", r.ChatHandler.CreateMessage)
		priv.PUT("/chat/rooms/:roomId/messages/:messageId", r.ChatHandler.UpdateMessage)
		priv.DELETE("/chat/rooms/:roomId/messages/:messageId", r.ChatHandler.DeleteMessage)
	}

	SetUpSwaggerRoutes(e)

	r.log.Info("chat command routes registered")
}

type RouterConfig struct {
	Port        string
	JWKSURL     string
	FrontendURL string
	DPoPStore   sharedauth.SetNXStore
}
