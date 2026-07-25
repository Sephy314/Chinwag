package router

import (
	"context"
	"log"
	"time"

	"github.com/Sephy314/chinwag/conn"
	"github.com/Sephy314/chinwag/conn/bridge"
	"github.com/Sephy314/chinwag/conn/cache"
	"github.com/Sephy314/chinwag/room/handler"
	"github.com/Sephy314/chinwag/room/repo"
	"github.com/Sephy314/chinwag/room/scheduler"
	"github.com/Sephy314/chinwag/room/service"
	"github.com/Sephy314/chinwag/shared/keyProvider"
	"github.com/google/uuid"
	echojwt "github.com/labstack/echo-jwt/v5"
	"github.com/labstack/echo/v5"
)

func SetUpRoomRouter(e *echo.Echo, user bridge.UserProvider) bridge.RoomMemberProvider {
	conns, err := conn.NewConnection()
	if err != nil {
		panic(err)
	}

	cacheRedis := cache.NewRedisCache(conns.Rds)

	roomRepo := repo.NewRoomRepo(conns.DB)
	roomMemberRepo := repo.NewRoomMemberRepo(conns.DB)
	unitOfWork := repo.NewSQLUnitOfWork(conns.DB)
	roomService := service.NewRoomService(roomRepo, unitOfWork)
	roomMemberService := service.NewRoomMemberService(roomMemberRepo, roomRepo, user, unitOfWork)
	inviteLinkService := service.NewInviteLinkService(cacheRedis, roomMemberService, user, roomRepo)
	roomHandler := handler.NewRoomHandler(roomService, roomMemberService)
	roomMemberHandler := handler.NewRoomMemberHandler(roomMemberService, roomService)
	inviteLinkHandler := handler.NewInviteLinkHandler(inviteLinkService)

	popScheduler := scheduler.NewPopScheduler(scheduler.NewSQLPopper(conns.DB), 1*time.Minute)
	go popScheduler.Start(context.Background())

	pub := e.Group("/rooms")
	{
		pub.GET("/health", roomHandler.Health)
		pub.GET("/:id", roomHandler.GetRoom)
	}

	e.GET("/users/:id/rooms", roomHandler.ListUserRooms)

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
		priv.PUT("/:id", roomHandler.UpdateRoom)
		priv.DELETE("/:id", roomHandler.DeleteRoom)

		priv.POST("/:roomId/members", roomMemberHandler.AddMember)
		priv.PUT("/:roomId/members/:userId", roomMemberHandler.UpdateMember)
		priv.DELETE("/:roomId/members/:userId", roomMemberHandler.RemoveMember)

		priv.GET("/:roomId/members", roomMemberHandler.ListMembers)
		priv.GET("/:roomId/members/:userId", roomMemberHandler.GetMember)

		priv.POST("/:roomId/invite", inviteLinkHandler.CreateInviteLink)
		priv.POST("/invite/:token/join", inviteLinkHandler.JoinByInviteLink)
	}

	log.Println("room routes registered")

	return newRoomMemberAdapter(roomMemberService)
}

func newRoomMemberAdapter(s *service.RoomMemberService) bridge.RoomMemberProvider {
	return bridge.NewRoomMemberAdapter(
		func(ctx context.Context, userId string) ([]bridge.RoomInfo, error) {
			uid, err := uuid.Parse(userId)
			if err != nil {
				return nil, err
			}
			rooms, err := s.GetRoomsByUserId(ctx, uid)
			if err != nil {
				return nil, err
			}
			result := make([]bridge.RoomInfo, len(rooms))
			for i, r := range rooms {
				result[i] = bridge.RoomInfo{
					Id:          r.Id.String(),
					Name:        r.Name,
					Description: r.Description,
					MaxMembers:  r.MaxMembers,
					OwnerId:     r.OwnerId.String(),
					PopAt:       r.PopAt,
					PoppedAt:    r.PoppedAt,
					CreatedAt:   r.CreatedAt,
					UpdatedAt:   r.UpdatedAt,
				}
			}
			return result, nil
		},
		func(ctx context.Context, roomId string) ([]bridge.RoomMemberInfo, error) {
			rid, err := uuid.Parse(roomId)
			if err != nil {
				return nil, err
			}
			members, err := s.GetUserByRoomId(ctx, rid)
			if err != nil {
				return nil, err
			}
			result := make([]bridge.RoomMemberInfo, len(members))
			for i, m := range members {
				result[i] = bridge.RoomMemberInfo{
					RoomId:   m.RoomId.String(),
					UserId:   m.UserId.String(),
					Role:     int(m.Role),
					JoinedAt: m.JoinedAt,
					LeftAt:   m.LeftAt,
				}
			}
			return result, nil
		},
	)
}
