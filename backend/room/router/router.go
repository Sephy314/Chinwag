package router

import (
	"log"

	"github.com/Sephy314/chinwag/conn"
	"github.com/Sephy314/chinwag/room/handler"
	"github.com/Sephy314/chinwag/room/repo"
	"github.com/Sephy314/chinwag/room/service"
	"github.com/Sephy314/chinwag/shared/keyProvider"
	echojwt "github.com/labstack/echo-jwt/v5"
	"github.com/labstack/echo/v5"
)

func SetUpRoomRouter(e *echo.Echo) {
	conns, err := conn.NewConnection()
	if err != nil {
		panic(err)
	}

	roomRepo := repo.NewRoomRepo(conns.DB)
	roomMemberRepo := repo.NewRoomMemberRepo(conns.DB)
	unitOfWork := repo.NewSQLUnitOfWork(conns.DB)
	roomService := service.NewRoomService(roomRepo, unitOfWork)
	roomMemberService := service.NewRoomMemberService(roomMemberRepo, unitOfWork)
	roomHandler := handler.NewRoomHandler(roomService, roomMemberService)
	roomMemberHandler := handler.NewRoomMemberHandler(roomMemberService, roomService)

	pub := e.Group("/room")
	{
		pub.GET("/health", roomHandler.Health)
		pub.GET("/:id", roomHandler.GetRoom)
		pub.GET("/owner/:ownerId", roomHandler.GetRoomsByOwnerId)
	}

	priv := e.Group("/room")
	priv.Use(echojwt.WithConfig(echojwt.Config{
		KeyFunc: keyProvider.KeyFunc,
	}))
	{
		priv.POST("", roomHandler.CreateRoom)
		priv.DELETE("/:id", roomHandler.DeleteRoom)

		priv.POST("/member/invite", roomMemberHandler.InviteUser)
		priv.POST("/member/kick", roomMemberHandler.KickUser)

		priv.GET("/member/room/:roomId", roomMemberHandler.GetUsersByRoomId)
		priv.GET("/member/user/:userId", roomMemberHandler.GetRoomsByUserId)

		priv.GET("/member/room/:roomId/user/:userId", roomMemberHandler.GetUserByRoomIdAndUserId)
	}

	log.Println("room routes registered")
}
