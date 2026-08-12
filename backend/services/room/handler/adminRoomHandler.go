package handler

import (
	"log/slog"
	"net/http"

	"github.com/Sephy314/chinwag/backend/services/room/service"
	"github.com/Sephy314/chinwag/backend/services/room/shared/errs"
	"github.com/Sephy314/chinwag/backend/services/room/shared/response"
	"github.com/Sephy314/chinwag/backend/services/room/structs"
	sharedauth "github.com/Sephy314/chinwag/backend/shared/auth"
	"github.com/google/uuid"
	"github.com/labstack/echo/v5"
)

type AdminRoomHandler struct {
	rooms   *service.RoomService
	members *service.RoomMemberService
	audit   *service.AuditClient
	log     *slog.Logger
}

func NewAdminRoomHandler(rooms *service.RoomService, members *service.RoomMemberService, audit *service.AuditClient, log *slog.Logger) *AdminRoomHandler {
	return &AdminRoomHandler{rooms: rooms, members: members, audit: audit, log: log}
}

func (h *AdminRoomHandler) adminID(c *echo.Context) string {
	claims, err := sharedauth.ClaimsFromContext(c)
	if err != nil {
		return ""
	}
	return claims.Subject
}

func (h *AdminRoomHandler) ListRooms(c *echo.Context) error {
	var req structs.ListRoomsRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, response.Error(err.Error()))
	}
	rooms, meta, err := h.rooms.AdminListRooms(c.Request().Context(), req)
	if err != nil {
		return c.JSON(errs.ParseError(err))
	}
	resp := response.OK(rooms)
	resp.Meta = meta
	return c.JSON(http.StatusOK, resp)
}

func (h *AdminRoomHandler) GetRoom(c *echo.Context) error {
	roomId, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, response.Error("invalid room id"))
	}
	room, err := h.rooms.AdminGetRoom(c.Request().Context(), roomId)
	if err != nil {
		return c.JSON(errs.ParseError(err))
	}
	return c.JSON(http.StatusOK, response.OK(room))
}

func (h *AdminRoomHandler) CreateRoom(c *echo.Context) error {
	var req structs.AdminCreateRoomRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, response.Error(err.Error()))
	}
	actor, err := uuid.Parse(h.adminID(c))
	if err != nil {
		return c.JSON(http.StatusUnauthorized, response.Error("invalid actor"))
	}
	room, err := h.rooms.AdminCreateRoom(c.Request().Context(), req, actor)
	if err != nil {
		h.log.Warn("admin room: create failed", "error", err)
		return c.JSON(errs.ParseError(err))
	}
	h.audit.Record(c.Request().Context(), h.adminID(c), "room.create", "room", room.Id.String(),
		map[string]any{"name": room.Name, "owner_id": room.OwnerId.String()})
	return c.JSON(http.StatusCreated, response.Created(room))
}

func (h *AdminRoomHandler) UpdateRoom(c *echo.Context) error {
	roomId, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, response.Error("invalid room id"))
	}
	var req structs.AdminUpdateRoomRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, response.Error(err.Error()))
	}
	room, err := h.rooms.AdminUpdateRoom(c.Request().Context(), roomId, req)
	if err != nil {
		h.log.Warn("admin room: update failed", "room_id", roomId.String(), "error", err)
		return c.JSON(errs.ParseError(err))
	}
	h.audit.Record(c.Request().Context(), h.adminID(c), "room.update", "room", room.Id.String(),
		map[string]any{"name": room.Name})
	return c.JSON(http.StatusOK, response.OK(room))
}

func (h *AdminRoomHandler) DeleteRoom(c *echo.Context) error {
	roomId, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, response.Error("invalid room id"))
	}
	if err := h.rooms.AdminDeleteRoom(c.Request().Context(), roomId); err != nil {
		h.log.Warn("admin room: delete failed", "room_id", roomId.String(), "error", err)
		return c.JSON(errs.ParseError(err))
	}
	h.audit.Record(c.Request().Context(), h.adminID(c), "room.delete", "room", roomId.String(), nil)
	return c.JSON(http.StatusOK, response.OK[any](nil))
}

func (h *AdminRoomHandler) ListMembers(c *echo.Context) error {
	roomId, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, response.Error("invalid room id"))
	}
	members, err := h.members.GetUserByRoomId(c.Request().Context(), roomId)
	if err != nil {
		return c.JSON(errs.ParseError(err))
	}
	return c.JSON(http.StatusOK, response.OK(members))
}

func (h *AdminRoomHandler) AddMember(c *echo.Context) error {
	roomId, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, response.Error("invalid room id"))
	}
	var body structs.AddRoomMemberRequest
	if err := c.Bind(&body); err != nil {
		return c.JSON(http.StatusBadRequest, response.Error(err.Error()))
	}
	member := structs.RoomUser{
		UserId: body.UserID,
		RoomId: roomId,
		Role:   body.Role,
	}
	if err := h.members.AdminInviteUser(c.Request().Context(), member); err != nil {
		h.log.Warn("admin room: add member failed", "room_id", roomId.String(), "user_id", body.UserID.String(), "error", err)
		return c.JSON(errs.ParseError(err))
	}
	h.audit.Record(c.Request().Context(), h.adminID(c), "member.add", "room", roomId.String(),
		map[string]any{"user_id": body.UserID.String(), "role": member.Role})
	return c.JSON(http.StatusCreated, response.OK[any](nil))
}

func (h *AdminRoomHandler) RemoveMember(c *echo.Context) error {
	roomId, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, response.Error("invalid room id"))
	}
	userId, err := uuid.Parse(c.Param("userId"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, response.Error("invalid user id"))
	}
	member := structs.RoomUser{UserId: userId, RoomId: roomId}
	if err := h.members.AdminKickUser(c.Request().Context(), member); err != nil {
		h.log.Warn("admin room: remove member failed", "room_id", roomId.String(), "user_id", userId.String(), "error", err)
		return c.JSON(errs.ParseError(err))
	}
	h.audit.Record(c.Request().Context(), h.adminID(c), "member.remove", "room", roomId.String(),
		map[string]any{"user_id": userId.String()})
	return c.JSON(http.StatusOK, response.OK[any](nil))
}

func (h *AdminRoomHandler) UpdateMember(c *echo.Context) error {
	roomId, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, response.Error("invalid room id"))
	}
	userId, err := uuid.Parse(c.Param("userId"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, response.Error("invalid user id"))
	}
	var req structs.UpdateRoomMemberRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, response.Error(err.Error()))
	}
	member, err := h.members.AdminUpdateRoomMember(c.Request().Context(), userId, roomId, req)
	if err != nil {
		h.log.Warn("admin room: update member failed", "room_id", roomId.String(), "user_id", userId.String(), "error", err)
		return c.JSON(errs.ParseError(err))
	}
	h.audit.Record(c.Request().Context(), h.adminID(c), "member.role_change", "room", roomId.String(),
		map[string]any{"user_id": userId.String(), "role": member.Role})
	return c.JSON(http.StatusOK, response.OK(member))
}

func (h *AdminRoomHandler) ListUserRooms(c *echo.Context) error {
	userId, err := uuid.Parse(c.Param("userId"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, response.Error("invalid user id"))
	}
	rooms, err := h.members.GetRoomsByUserId(c.Request().Context(), userId)
	if err != nil {
		return c.JSON(errs.ParseError(err))
	}
	return c.JSON(http.StatusOK, response.OK(rooms))
}

func (h *AdminRoomHandler) StatsRooms(c *echo.Context) error {
	n, err := h.rooms.AdminCountRooms(c.Request().Context())
	if err != nil {
		return c.JSON(errs.ParseError(err))
	}
	return c.JSON(http.StatusOK, response.OK(map[string]int64{"total_rooms": n}))
}
