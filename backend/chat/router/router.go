package router

import (
	"log"

	"github.com/Sephy314/chinwag/chat/handler"
	"github.com/Sephy314/chinwag/chat/repo"
	"github.com/Sephy314/chinwag/chat/service"
	"github.com/Sephy314/chinwag/conn"
	"github.com/Sephy314/chinwag/conn/bridge"
	"github.com/google/uuid"
	echojwt "github.com/labstack/echo-jwt/v5"
	"github.com/labstack/echo/v5"

	"github.com/Sephy314/chinwag/shared/keyProvider"
)

func SetUpChatRouter(e *echo.Echo, user bridge.UserProvider, member bridge.RoomMemberProvider) {
	conns, err := conn.NewConnection()
	if err != nil {
		panic(err)
	}

	chatRepoImpl := repo.NewChatRepo(conns.DB)
	unitOfWork := repo.NewSQLUnitOfWork(conns.DB)

	hub := handler.NewHub()
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
			log.Println(err)
			return echo.ErrUnauthorized
		},
	}))
	{
		priv.POST("/rooms/:roomId/messages", chatH.CreateMessage)
		priv.GET("/rooms/:roomId/messages", chatH.ListMessages)
		priv.GET("/rooms/:roomId/messages/:messageId", chatH.GetMessage)
		priv.PUT("/rooms/:roomId/messages/:messageId", chatH.UpdateMessage)
		priv.DELETE("/rooms/:roomId/messages/:messageId", chatH.DeleteMessage)
	}

	log.Println("chat routes registered")
}
