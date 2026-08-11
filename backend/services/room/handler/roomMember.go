package handler

import (
	"log/slog"
	"net/http"

	"github.com/Sephy314/chinwag/backend/services/room/service"
	"github.com/Sephy314/chinwag/backend/services/room/shared/errs"
	"github.com/Sephy314/chinwag/backend/services/room/shared/response"
	"github.com/Sephy314/chinwag/backend/services/room/shared/utils"
	"github.com/Sephy314/chinwag/backend/services/room/structs"
	"github.com/google/uuid"
	"github.com/labstack/echo/v5"
)

type RoomMemberHandler interface {
	AddMember(c *echo.Context) error
	RemoveMember(c *echo.Context) error
	ListMembers(c *echo.Context) error
	GetMember(c *echo.Context) error
	UpdateMember(c *echo.Context) error
}

type RoomMemberHandlerImpl struct {
	service     service.RoomMemberServiceInterface
	roomService service.RoomServiceInterface
	user        service.UserProvider
	log         *slog.Logger
}

func NewRoomMemberHandler(s service.RoomMemberServiceInterface, roomService service.RoomServiceInterface, user service.UserProvider, log ...*slog.Logger) *RoomMemberHandlerImpl {
	l := slog.Default()
	if len(log) > 0 && log[0] != nil {
		l = log[0]
	}
	return &RoomMemberHandlerImpl{
		service:     s,
		roomService: roomService,
		user:        user,
		log:         l,
	}
}

func (h *RoomMemberHandlerImpl) AddMember(c *echo.Context) error {
	roomId, err := uuid.Parse(c.Param("roomId"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, response.Error("invalid room id"))
	}

	var body structs.AddRoomMemberRequest
	if err := c.Bind(&body); err != nil {
		return c.JSON(http.StatusBadRequest, response.Error(err.Error()))
	}

	ok, err := utils.IsManager(c, roomId, h.service)
	if err != nil {
		return c.JSON(errs.ParseError(err))
	}

	if !ok {
		return c.JSON(http.StatusForbidden, response.Error("Admin permission is required"))
	}

	req := structs.RoomUser{
		UserId: body.UserID,
		RoomId: roomId,
		Role:   body.Role,
	}

	if err := h.service.InviteUser(c.Request().Context(), req); err != nil {
		h.log.Warn("room: add member failed", "room_id", roomId.String(), "user_id", body.UserID, "error", err)
		return c.JSON(errs.ParseError(err))
	}

	h.log.Info("room: member added", "room_id", roomId.String(), "user_id", body.UserID, "role", body.Role)
	return c.JSON(http.StatusCreated, response.OK[any](nil))
}

func (h *RoomMemberHandlerImpl) RemoveMember(c *echo.Context) error {
	roomId, err := uuid.Parse(c.Param("roomId"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, response.Error("invalid room id"))
	}

	userId, err := uuid.Parse(c.Param("userId"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, response.Error("invalid user id"))
	}

	ok, err := utils.IsManager(c, roomId, h.service)
	if err != nil {
		return c.JSON(errs.ParseError(err))
	}

	if !ok {
		return c.JSON(http.StatusForbidden, response.Error("Admin permission is required"))
	}

	req := structs.RoomUser{
		UserId: userId,
		RoomId: roomId,
	}

	if err := h.service.KickUser(c.Request().Context(), req); err != nil {
		h.log.Warn("room: remove member failed", "room_id", roomId.String(), "user_id", userId.String(), "error", err)
		return c.JSON(errs.ParseError(err))
	}

	h.log.Info("room: member removed", "room_id", roomId.String(), "user_id", userId.String())
	return c.JSON(http.StatusOK, response.OK[any](nil))
}

func (h *RoomMemberHandlerImpl) ListMembers(c *echo.Context) error {
	roomId, err := uuid.Parse(c.Param("roomId"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, response.Error("invalid room id"))
	}

	members, err := h.service.GetUserByRoomId(c.Request().Context(), roomId)
	if err != nil {
		return c.JSON(errs.ParseError(err))
	}

	result := make([]structs.RoomMemberResponse, len(members))
	for i, m := range members {
		r := structs.RoomMemberResponse{
			RoomId:   m.RoomId.String(),
			UserId:   m.UserId.String(),
			Role:     int(m.Role),
			JoinedAt: m.JoinedAt,
			LeftAt:   m.LeftAt,
		}
		if h.user != nil {
			if user, err := h.user.GetUser(c.Request().Context(), m.UserId.String()); err == nil {
				r.UserName = user.Name
			}
		}
		result[i] = r
	}

	return c.JSON(http.StatusOK, response.OK(result))
}

func (h *RoomMemberHandlerImpl) GetMember(c *echo.Context) error {
	roomId, err := uuid.Parse(c.Param("roomId"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, response.Error("invalid room id"))
	}

	userId, err := uuid.Parse(c.Param("userId"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, response.Error("invalid user id"))
	}

	member, err := h.service.GetUserByRoomIdAndUserId(c.Request().Context(), userId, roomId)
	if err != nil {
		return c.JSON(errs.ParseError(err))
	}

	return c.JSON(http.StatusOK, response.OK(member))
}

func (h *RoomMemberHandlerImpl) UpdateMember(c *echo.Context) error {
	roomId, err := uuid.Parse(c.Param("roomId"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, response.Error("invalid room id"))
	}

	userId, err := uuid.Parse(c.Param("userId"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, response.Error("invalid user id"))
	}

	ok, err := utils.IsManager(c, roomId, h.service)
	if err != nil {
		return c.JSON(errs.ParseError(err))
	}

	if !ok {
		return c.JSON(http.StatusForbidden, response.Error("Admin permission is required"))
	}

	var req structs.UpdateRoomMemberRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, response.Error(err.Error()))
	}

	member, err := h.service.UpdateRoomMember(c.Request().Context(), userId, roomId, req)
	if err != nil {
		h.log.Warn("room: update member failed", "room_id", roomId.String(), "user_id", userId.String(), "error", err)
		return c.JSON(errs.ParseError(err))
	}

	h.log.Info("room: member updated", "room_id", roomId.String(), "user_id", userId.String())
	return c.JSON(http.StatusOK, response.OK(member))
}

var _ RoomMemberHandler = (*RoomMemberHandlerImpl)(nil)
