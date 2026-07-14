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

	pub := e.Group("/rooms")
	{
		pub.GET("/health", roomHandler.Health)
		pub.GET("", roomHandler.ListRooms)
		pub.GET("/:id", roomHandler.GetRoom)
	}

	priv := e.Group("/rooms")
	priv.Use(echojwt.WithConfig(echojwt.Config{
		KeyFunc: keyProvider.KeyFunc,
		ErrorHandler: func(c *echo.Context, err error) error {
			log.Println(err)
			return echo.ErrUnauthorized
		},
	}))
	{
		priv.POST("", roomHandler.CreateRoom)
		priv.DELETE("/:id", roomHandler.DeleteRoom)

		priv.POST("/:roomId/members", roomMemberHandler.AddMember)
		priv.DELETE("/:roomId/members/:userId", roomMemberHandler.RemoveMember)

		priv.GET("/:roomId/members", roomMemberHandler.ListMembers)
		priv.GET("/:roomId/members/:userId", roomMemberHandler.GetMember)
	}

	log.Println("room routes registered")
}
