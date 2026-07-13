package handler

import (
	"net/http"

	"github.com/Sephy314/chinwag/room/service"
	"github.com/Sephy314/chinwag/room/structs"
	"github.com/Sephy314/chinwag/shared/errs"
	"github.com/google/uuid"
	"github.com/labstack/echo/v5"
)

type RoomHandler interface {
	Health(c *echo.Context) error
	CreateRoom(c *echo.Context) error
	GetRoom(c *echo.Context) error
	GetRoomsByOwnerId(c *echo.Context) error
	DeleteRoom(c *echo.Context) error
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
	return c.JSON(http.StatusOK, map[string]string{
		"message": "ok",
	})
}

func (h *RoomHandlerImpl) CreateRoom(c *echo.Context) error {
	var req structs.CreateRoomRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": err.Error(),
		})
	}
	//
	//uid, err := utils.GetUserIdByEchoContext(c)
	//
	//if err != nil {
	//	return c.JSON(errs.ParseError(err))
	//}

	room, err := h.service.CreateRoom(c.Request().Context(), req)
	if err != nil {
		return c.JSON(errs.ParseError(err))
	}

	return c.JSON(http.StatusCreated, room)
}

func (h *RoomHandlerImpl) GetRoom(c *echo.Context) error {
	roomId, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "invalid room id",
		})
	}

	room, err := h.service.GetRoomById(c.Request().Context(), roomId)
	if err != nil {
		return c.JSON(errs.ParseError(err))
	}

	return c.JSON(http.StatusOK, room)
}

func (h *RoomHandlerImpl) GetRoomsByOwnerId(c *echo.Context) error {
	ownerId, err := uuid.Parse(c.Param("ownerId"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "invalid owner id",
		})
	}

	rooms, err := h.service.GetRoomsByOwnerId(c.Request().Context(), ownerId)
	if err != nil {
		return c.JSON(errs.ParseError(err))
	}

	return c.JSON(http.StatusOK, rooms)
}

func (h *RoomHandlerImpl) DeleteRoom(c *echo.Context) error {
	roomId, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "invalid room id",
		})
	}

	if err := h.service.DeleteRoom(c.Request().Context(), roomId); err != nil {
		return c.JSON(errs.ParseError(err))
	}

	return c.JSON(http.StatusOK, map[string]string{
		"message": "ok",
	})
}

var _ RoomHandler = (*RoomHandlerImpl)(nil)
