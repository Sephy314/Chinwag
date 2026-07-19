package handler

import (
	"context"
	"net/http"

	"github.com/Sephy314/chinwag/room/service"
	"github.com/Sephy314/chinwag/room/structs"
	"github.com/Sephy314/chinwag/shared/errs"
	"github.com/Sephy314/chinwag/shared/response"
	"github.com/Sephy314/chinwag/shared/utils"
	"github.com/google/uuid"
	"github.com/labstack/echo/v5"
)

type RoomHandler interface {
	Health(c *echo.Context) error
	CreateRoom(c *echo.Context) error
	GetRoom(c *echo.Context) error
	ListRooms(c *echo.Context) error
	UpdateRoom(c *echo.Context) error
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

// Health godoc
// @Summary      Health check
// @Description  Check the health status of the room service
// @Tags         room
// @Produce      json
// @Success      200 {object} response.Response[any]
// @Router       /rooms/health [get]
func (h *RoomHandlerImpl) Health(c *echo.Context) error {
	return c.JSON(http.StatusOK, response.OK[any](nil))
}

// CreateRoom godoc
// @Summary      Create a chat room
// @Description  Create a new chat room. The owner is the authenticated user. Returns the created room with generated UUID and timestamps.
// @Tags         room
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        request body structs.CreateRoomRequest true "Room creation info" example({"name":"general","description":"General chat room","max_members":50})
// @Success      201 {object} response.Response[domain.Room] "Created room with fields: id (UUID), name, description, max_members, owner_id, created_at, updated_at"
// @Failure      400 {object} response.Response[any] "Invalid request body or validation error"
// @Failure      500 {object} response.Response[any] "Internal server error"
// @Router       /rooms [post]
func (h *RoomHandlerImpl) CreateRoom(c *echo.Context) error {
	var req structs.CreateRoomRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, response.Error( err.Error()))
	}

	ownerId, err := utils.GetUserIdByEchoContext(c)

	if err != nil {
		return echo.ErrUnauthorized
	}

	ctx := context.WithValue(c.Request().Context(), "ownerId", ownerId)

	room, err := h.service.CreateRoom(ctx, req)
	if err != nil {
		return c.JSON(errs.ParseError(err))
	}

	return c.JSON(http.StatusCreated, response.Created(room))
}

// GetRoom godoc
// @Summary      Get a chat room
// @Description  Retrieve chat room information by its UUID.
// @Tags         room
// @Produce      json
// @Param        id path string true "Room UUID" example(550e8400-e29b-41d4-a716-446655440000)
// @Success      200 {object} response.Response[domain.Room] "Room found"
// @Failure      400 {object} response.Response[any] "Invalid UUID format"
// @Failure      404 {object} response.Response[any] "Room not found"
// @Router       /rooms/{id} [get]
func (h *RoomHandlerImpl) GetRoom(c *echo.Context) error {
	roomId, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, response.Error( "invalid room id"))
	}

	room, err := h.service.GetRoomById(c.Request().Context(), roomId)
	if err != nil {
		return c.JSON(errs.ParseError(err))
	}

	return c.JSON(http.StatusOK, response.OK(room))
}

// ListRooms godoc
// @Summary      List rooms
// @Description  Retrieve rooms filtered by ownerId (rooms owned) or memberId (rooms joined). Pass exactly one query parameter.
// @Tags         room
// @Produce      json
// @Param        ownerId query string false "Filter by owner UUID" example(550e8400-e29b-41d4-a716-446655440000)
// @Param        memberId query string false "Filter by member UUID" example(550e8400-e29b-41d4-a716-446655440000)
// @Success      200 {object} response.Response[[]domain.Room] "Array of rooms"
// @Failure      400 {object} response.Response[any] "Invalid UUID format or missing query parameter"
// @Router       /rooms [get]
func (h *RoomHandlerImpl) ListRooms(c *echo.Context) error {
	ownerIdStr := c.QueryParam("ownerId")
	memberIdStr := c.QueryParam("memberId")

	if ownerIdStr != "" && memberIdStr != "" {
		return c.JSON(http.StatusBadRequest, response.Error( "provide only one of ownerId or memberId"))
	}

	if ownerIdStr == "" && memberIdStr == "" {
		return c.JSON(http.StatusBadRequest, response.Error( "provide ownerId or memberId query parameter"))
	}

	ctx := c.Request().Context()

	if ownerIdStr != "" {
		ownerId, err := uuid.Parse(ownerIdStr)
		if err != nil {
			return c.JSON(http.StatusBadRequest, response.Error( "invalid ownerId"))
		}
		rooms, err := h.service.GetRoomsByOwnerId(ctx, ownerId)
		if err != nil {
			return c.JSON(errs.ParseError(err))
		}
		return c.JSON(http.StatusOK, response.OK(rooms))
	}

	memberId, err := uuid.Parse(memberIdStr)
	if err != nil {
		return c.JSON(http.StatusBadRequest, response.Error( "invalid memberId"))
	}
	rooms, err := h.memberService.GetRoomsByUserId(ctx, memberId)
	if err != nil {
		return c.JSON(errs.ParseError(err))
	}
	return c.JSON(http.StatusOK, response.OK(rooms))
}

// UpdateRoom godoc
// @Summary      Update a chat room
// @Description  Update room information by UUID. Only provided fields are updated; omitted fields remain unchanged.
// @Tags         room
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id path string true "Room UUID" example(550e8400-e29b-41d4-a716-446655440000)
// @Param        request body structs.UpdateRoomRequest true "Fields to update" example({"name":"new name","max_members":100})
// @Success      200 {object} response.Response[domain.Room] "Successfully updated room"
// @Failure      400 {object} response.Response[any] "Invalid request body or UUID format"
// @Failure      404 {object} response.Response[any] "Room not found"
// @Router       /rooms/{id} [put]
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

// DeleteRoom godoc
// @Summary      Delete a chat room
// @Description  Soft-delete a chat room by UUID. Only the room owner or an admin should call this endpoint.
// @Tags         room
// @Produce      json
// @Security     BearerAuth
// @Param        id path string true "Room UUID" example(550e8400-e29b-41d4-a716-446655440000)
// @Success      200 {object} response.Response[any]
// @Failure      400 {object} response.Response[any] "Invalid UUID format"
// @Failure      500 {object} response.Response[any] "Internal server error"
// @Router       /rooms/{id} [delete]
func (h *RoomHandlerImpl) DeleteRoom(c *echo.Context) error {
	roomId, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, response.Error( "invalid room id"))
	}

	if err := h.service.DeleteRoom(c.Request().Context(), roomId); err != nil {
		return c.JSON(errs.ParseError(err))
	}

	return c.JSON(http.StatusOK, response.OK[any](nil))
}

var _ RoomHandler = (*RoomHandlerImpl)(nil)
