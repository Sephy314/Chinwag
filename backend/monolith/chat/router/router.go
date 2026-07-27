package router

import (
	"time"

	"github.com/Sephy314/chinwag/backend/monolith/chat/handler"
	"github.com/Sephy314/chinwag/backend/monolith/chat/repo"
	"github.com/Sephy314/chinwag/backend/monolith/chat/service"
	"github.com/Sephy314/chinwag/backend/monolith/conn"
	"github.com/Sephy314/chinwag/backend/monolith/conn/bridge"
	appMiddleware "github.com/Sephy314/chinwag/backend/monolith/middleware"
	"github.com/Sephy314/chinwag/backend/monolith/shared/keyProvider"
	"github.com/Sephy314/chinwag/backend/monolith/shared/logger"
	"github.com/google/uuid"
	echojwt "github.com/labstack/echo-jwt/v5"
	"github.com/labstack/echo/v5"
)

func SetUpChatRouter(e *echo.Echo, user bridge.UserProvider, member bridge.RoomMemberProvider, log logger.Logger) {
	conns, err := conn.NewConnection()
	if err != nil {
		panic(err)
	}

	chatRepoImpl := repo.NewChatRepo(conns.DB)
	unitOfWork := repo.NewSQLUnitOfWork(conns.DB)

	hub := handler.NewHub(log)
	go hub.Run()

	broadcastFn := func(roomId uuid.UUID, event []byte) {
		hub.Broadcast(roomId, event)
	}

	chatSvc := service.NewChatService(chatRepoImpl, unitOfWork, user, member, broadcastFn)
	chatH := handler.NewChatHandler(chatSvc)

	pub := e.Group("/chat")
	{
		pub.GET("/health", chatH.Health)
		pub.GET("/rooms/:roomId/ws", hub.ServeWS)
	}

	priv := e.Group("/chat")
	priv.Use(echojwt.WithConfig(echojwt.Config{
		KeyFunc: keyProvider.KeyFunc,
		ErrorHandler: func(c *echo.Context, err error) error {
			log.Error("jwt error", "error", err)
			return echo.ErrUnauthorized
		},
	}))
	{
		// Rate limit message creation: 30 requests per minute per user
		msgRateLimitStore := appMiddleware.NewRedisSlidingWindowStore(conns.Rds, 30, time.Minute)
		msgRateLimit := appMiddleware.NewRateLimitMiddleware(msgRateLimitStore, appMiddleware.JWTUserExtractor)

		priv.POST("/rooms/:roomId/messages", chatH.CreateMessage, msgRateLimit)
		priv.GET("/rooms/:roomId/messages", chatH.ListMessages)
		priv.GET("/rooms/:roomId/messages/:messageId", chatH.GetMessage)
		priv.PUT("/rooms/:roomId/messages/:messageId", chatH.UpdateMessage)
		priv.DELETE("/rooms/:roomId/messages/:messageId", chatH.DeleteMessage)
	}

	log.Info("chat routes registered")
}
