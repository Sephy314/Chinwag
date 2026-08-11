package router

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/Sephy314/chinwag/backend/services/room/handler"
	sharedauth "github.com/Sephy314/chinwag/backend/shared/auth"
	"github.com/Sephy314/chinwag/backend/shared/auth/dpop"
	"github.com/labstack/echo/v5"
	"github.com/labstack/echo/v5/middleware"
)

type Router struct {
	Echo                *echo.Echo
	RoomHandler         *handler.RoomHandlerImpl
	RoomMemberHandler   *handler.RoomMemberHandlerImpl
	InviteLinkHandler   *handler.InviteLinkHandlerImpl
	log                 *slog.Logger
}

func NewRouter(
	roomHandler *handler.RoomHandlerImpl,
	roomMemberHandler *handler.RoomMemberHandlerImpl,
	inviteLinkHandler *handler.InviteLinkHandlerImpl,
	log *slog.Logger,
) *Router {
	return &Router{
		Echo:              echo.New(),
		RoomHandler:       roomHandler,
		RoomMemberHandler: roomMemberHandler,
		InviteLinkHandler: inviteLinkHandler,
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
		pub.GET("/health", r.RoomHandler.Health)
		pub.GET("/rooms/health", r.RoomHandler.Health)
		pub.GET("/rooms/owner/:ownerId", r.RoomHandler.ListRoomsByOwnerId)
		pub.GET("/rooms/member/:memberId", r.RoomHandler.ListRoomsByMemberId)
		pub.GET("/rooms/:id", r.RoomHandler.GetRoom)
		pub.GET("/users/:id/rooms", r.RoomHandler.ListUserRooms)
		pub.GET("/rooms/:roomId/members", r.RoomMemberHandler.ListMembers)
		pub.GET("/rooms/:roomId/members/:userId", r.RoomMemberHandler.GetMember)
	}

	priv := e.Group("")
	priv.Use(sharedauth.NewMiddleware(jwksClient, r.log, cfg.DPoPValidator))
	{
		priv.POST("/rooms", r.RoomHandler.CreateRoom)
		priv.PUT("/rooms/:id", r.RoomHandler.UpdateRoom)
		priv.DELETE("/rooms/:id", r.RoomHandler.DeleteRoom)
		priv.POST("/rooms/:id/pop", r.RoomHandler.PopRoom)

		priv.POST("/rooms/:roomId/members", r.RoomMemberHandler.AddMember)
		priv.PUT("/rooms/:roomId/members/:userId", r.RoomMemberHandler.UpdateMember)
		priv.DELETE("/rooms/:roomId/members/:userId", r.RoomMemberHandler.RemoveMember)

		priv.POST("/rooms/:roomId/invite", r.InviteLinkHandler.CreateInviteLink)
		priv.POST("/rooms/invite/:token/join", r.InviteLinkHandler.JoinByInviteLink)
	}

	r.SetUpSwaggerRoutes(e)

	r.log.Info("room routes registered")
}

type RouterConfig struct {
	Port          string
	JWKSURL       string
	FrontendURL   string
	DPoPValidator *dpop.Validator
}
