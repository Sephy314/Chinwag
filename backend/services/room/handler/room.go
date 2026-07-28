package handler

import (
	"context"
	"net/http"

	"github.com/Sephy314/chinwag/backend/services/room/domain"
	"github.com/Sephy314/chinwag/backend/services/room/service"
	"github.com/Sephy314/chinwag/backend/services/room/structs"
	"github.com/Sephy314/chinwag/backend/services/room/shared/errs"
	"github.com/Sephy314/chinwag/backend/services/room/shared/response"
	"github.com/Sephy314/chinwag/backend/services/room/shared/utils"
	"github.com/google/uuid"
	"github.com/labstack/echo/v5"
)

type RoomHandler interface {
	Health(c *echo.Context) error
	CreateRoom(c *echo.Context) error
	GetRoom(c *echo.Context) error
	ListRoomsByOwnerId(c *echo.Context) error
	ListRoomsByMemberId(c *echo.Context) error
	ListUserRooms(c *echo.Context) error
	UpdateRoom(c *echo.Context) error
	DeleteRoom(c *echo.Context) error
	PopRoom(c *echo.Context) error
}

type RoomHandlerImpl struct {
	service       service.RoomServiceInterface
	memberService service.RoomMemberServiceInterface
}

func NewRoomHandler(s service.RoomServiceInterface, memberService service.RoomMemberServiceInterface) *RoomHandlerImpl {
	return &RoomHandlerImpl{
		service:       s,
		memberService: memberService,
	}
}

func (h *RoomHandlerImpl) Health(c *echo.Context) error {
	return c.JSON(http.StatusOK, response.OK[any](nil))
}

func (h *RoomHandlerImpl) CreateRoom(c *echo.Context) error {
	var req structs.CreateRoomRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, response.Error(err.Error()))
	}

	ownerIdStr, err := utils.GetUserIdByEchoContext(c)

	if err != nil {
		return echo.ErrUnauthorized
	}

	ownerId, err := uuid.Parse(*ownerIdStr)

	if err != nil {
		return c.JSON(http.StatusBadRequest, response.Error("invalid user id"))
	}

	ctx := context.WithValue(c.Request().Context(), "ownerId", ownerId)

	room, err := h.service.CreateRoom(ctx, req)
	if err != nil {
		return c.JSON(errs.ParseError(err))
	}

	return c.JSON(http.StatusCreated, response.Created(room))
}

func (h *RoomHandlerImpl) GetRoom(c *echo.Context) error {
	roomId, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, response.Error("invalid room id"))
	}

	room, err := h.service.GetRoomById(c.Request().Context(), roomId)
	if err != nil {
		return c.JSON(errs.ParseError(err))
	}

	return c.JSON(http.StatusOK, response.OK(room))
}

func (h *RoomHandlerImpl) ListRoomsByOwnerId(c *echo.Context) error {
	ownerId, err := uuid.Parse(c.Param("ownerId"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, response.Error("invalid owner id"))
	}

	rooms, err := h.service.GetRoomsByOwnerId(c.Request().Context(), ownerId)
	if err != nil {
		return c.JSON(errs.ParseError(err))
	}

	return c.JSON(http.StatusOK, response.OK(rooms))
}

func (h *RoomHandlerImpl) ListRoomsByMemberId(c *echo.Context) error {
	memberId, err := uuid.Parse(c.Param("memberId"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, response.Error("invalid member id"))
	}

	rooms, err := h.memberService.GetRoomsByUserId(c.Request().Context(), memberId)
	if err != nil {
		return c.JSON(errs.ParseError(err))
	}

	return c.JSON(http.StatusOK, response.OK(rooms))
}

func (h *RoomHandlerImpl) ListUserRooms(c *echo.Context) error {
	userId, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, response.Error("invalid user id"))
	}

	ctx := c.Request().Context()

	ownedRooms, err := h.service.GetRoomsByOwnerId(ctx, userId)
	if err != nil {
		return c.JSON(errs.ParseError(err))
	}

	joinedRooms, err := h.memberService.GetRoomsByUserId(ctx, userId)
	if err != nil {
		return c.JSON(errs.ParseError(err))
	}

	seen := make(map[uuid.UUID]bool)
	var all []domain.Room
	for _, room := range ownedRooms {
		seen[room.Id] = true
		all = append(all, room)
	}
	for _, room := range joinedRooms {
		if !seen[room.Id] {
			all = append(all, room)
		}
	}

	return c.JSON(http.StatusOK, response.OK(all))
}

func (h *RoomHandlerImpl) UpdateRoom(c *echo.Context) error {
	roomId, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, response.Error("invalid room id"))
	}

	var req structs.UpdateRoomRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, response.Error(err.Error()))
	}

	room, err := h.service.UpdateRoom(c.Request().Context(), roomId, req)
	if err != nil {
		return c.JSON(errs.ParseError(err))
	}

	return c.JSON(http.StatusOK, response.OK(room))
}

func (h *RoomHandlerImpl) DeleteRoom(c *echo.Context) error {
	roomId, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, response.Error("invalid room id"))
	}

	if err := h.service.DeleteRoom(c.Request().Context(), roomId); err != nil {
		return c.JSON(errs.ParseError(err))
	}

	return c.JSON(http.StatusOK, response.OK[any](nil))
}

func (h *RoomHandlerImpl) PopRoom(c *echo.Context) error {
	roomId, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, response.Error("invalid room id"))
	}

	userIdStr, err := utils.GetUserIdByEchoContext(c)
	if err != nil {
		return echo.ErrUnauthorized
	}

	userId, err := uuid.Parse(*userIdStr)
	if err != nil {
		return c.JSON(http.StatusBadRequest, response.Error("invalid user id"))
	}

	ok, err := h.memberService.HasManagerPermission(c.Request().Context(), userId, roomId)
	if err != nil {
		return c.JSON(errs.ParseError(err))
	}
	if !ok {
		return c.JSON(http.StatusForbidden, response.Error("Admin permission is required"))
	}

	if err := h.service.PopRoom(c.Request().Context(), roomId); err != nil {
		return c.JSON(errs.ParseError(err))
	}

	return c.JSON(http.StatusOK, response.OK[any](nil))
}

var _ RoomHandler = (*RoomHandlerImpl)(nil)
