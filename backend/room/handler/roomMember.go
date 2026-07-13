package handler

import (
	"net/http"

	"github.com/Sephy314/chinwag/room/service"
	"github.com/Sephy314/chinwag/room/structs"
	"github.com/Sephy314/chinwag/shared/errs"
	"github.com/Sephy314/chinwag/shared/utils"
	"github.com/google/uuid"
	"github.com/labstack/echo/v5"
)

type RoomMemberHandler interface {
	InviteUser(c *echo.Context) error
	KickUser(c *echo.Context) error
	GetUsersByRoomId(c *echo.Context) error
	GetRoomsByUserId(c *echo.Context) error
	GetUserByRoomIdAndUserId(c *echo.Context) error
}

type RoomMemberHandlerImpl struct {
	service     service.RoomMemberServiceInterface
	roomService service.RoomServiceInterface
}

func NewRoomMemberHandler(s service.RoomMemberServiceInterface, roomService service.RoomServiceInterface) *RoomMemberHandlerImpl {
	return &RoomMemberHandlerImpl{
		service:     s,
		roomService: roomService,
	}
}

func (h *RoomMemberHandlerImpl) InviteUser(c *echo.Context) error {
	var req structs.RoomUser
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": err.Error(),
		})
	}

	ok, err := utils.IsManager(c, req.RoomId, h.service)
	if err != nil {
		return c.JSON(errs.ParseError(err))
	}

	if !ok {
		return c.JSON(http.StatusForbidden, map[string]string{
			"error": "Manager Permission is required",
		})
	}

	if err := h.service.InviteUser(c.Request().Context(), req); err != nil {
		return c.JSON(errs.ParseError(err))
	}

	return c.JSON(http.StatusCreated, map[string]string{
		"message": "ok",
	})
}

func (h *RoomMemberHandlerImpl) KickUser(c *echo.Context) error {
	var req structs.RoomUser
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": err.Error(),
		})
	}

	if err := h.service.KickUser(c.Request().Context(), req); err != nil {
		return c.JSON(errs.ParseError(err))
	}

	return c.JSON(http.StatusOK, map[string]string{
		"message": "ok",
	})
}

func (h *RoomMemberHandlerImpl) GetUsersByRoomId(c *echo.Context) error {
	roomId, err := uuid.Parse(c.Param("roomId"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "invalid room id",
		})
	}

	members, err := h.service.GetUserByRoomId(c.Request().Context(), roomId)
	if err != nil {
		return c.JSON(errs.ParseError(err))
	}

	return c.JSON(http.StatusOK, members)
}

func (h *RoomMemberHandlerImpl) GetRoomsByUserId(c *echo.Context) error {
	userId, err := uuid.Parse(c.Param("userId"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "invalid user id",
		})
	}

	rooms, err := h.service.GetRoomsByUserId(c.Request().Context(), userId)
	if err != nil {
		return c.JSON(errs.ParseError(err))
	}

	return c.JSON(http.StatusOK, rooms)
}

func (h *RoomMemberHandlerImpl) GetUserByRoomIdAndUserId(c *echo.Context) error {
	roomId, err := uuid.Parse(c.Param("roomId"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "invalid room id",
		})
	}

	userId, err := uuid.Parse(c.Param("userId"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "invalid user id",
		})
	}

	member, err := h.service.GetUserByRoomIdAndUserId(c.Request().Context(), userId, roomId)
	if err != nil {
		return c.JSON(errs.ParseError(err))
	}

	return c.JSON(http.StatusOK, member)
}

var _ RoomMemberHandler = (*RoomMemberHandlerImpl)(nil)
