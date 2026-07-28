package router

import (
	"os"
	"time"

	"github.com/Sephy314/chinwag/backend/monolith/chat/handler"
	"github.com/Sephy314/chinwag/backend/monolith/chat/repo"
	"github.com/Sephy314/chinwag/backend/monolith/chat/service"
	"github.com/Sephy314/chinwag/backend/monolith/conn"
	appMiddleware "github.com/Sephy314/chinwag/backend/monolith/middleware"
	"github.com/Sephy314/chinwag/backend/monolith/shared/logger"
	sharedauth "github.com/Sephy314/chinwag/backend/shared/auth"
	"github.com/google/uuid"
	"github.com/labstack/echo/v5"
)

func SetUpChatRouter(e *echo.Echo, user service.UserProvider, member service.RoomMemberProvider, log logger.Logger) {
	conns, err := conn.NewConnection()
	if err != nil {
		panic(err)
	}

	chatRepoImpl := repo.NewChatRepo(conns.DB)
	unitOfWork := repo.NewSQLUnitOfWork(conns.DB)

	jwksURL := os.Getenv("AUTH_JWKS_URL")
	if jwksURL == "" {
		jwksURL = "http://localhost:8081/.well-known/jwks.json"
	}
	jwksClient := sharedauth.NewJWKSClient(jwksURL, 5*time.Minute)

	hub := handler.NewHub(log, jwksClient)
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
	priv.Use(sharedauth.NewMiddleware(jwksClient))
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
