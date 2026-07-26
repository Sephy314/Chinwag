package handler

import (
	"context"
	"net/http"

	"github.com/Sephy314/chinwag/room/domain"
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
// @Param        request body structs.CreateRoomRequest true "Room creation info"
// @Success      201 {object} response.Response[domain.Room] "Created room with fields: id (UUID), name, description, max_members, owner_id, created_at, updated_at"
// @Failure      400 {object} response.Response[any] "Invalid request body or validation error"
// @Failure      500 {object} response.Response[any] "Internal server error"
// @Router       /rooms [post]
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

// GetRoom godoc
// @Summary      Get a chat room
// @Description  Retrieve chat room information by its UUID.
// @Tags         room
// @Produce      json
// @Param        id path string true "Room UUID"
// @Success      200 {object} response.Response[domain.Room] "Room found"
// @Failure      400 {object} response.Response[any] "Invalid UUID format"
// @Failure      404 {object} response.Response[any] "Room not found"
// @Router       /rooms/{id} [get]
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

// ListRoomsByOwnerId godoc
// @Summary      List rooms by owner
// @Description  Retrieve all rooms owned by a specific user.
// @Tags         room
// @Produce      json
// @Param        ownerId path string true "Owner UUID"
// @Success      200 {object} response.Response[[]domain.Room] "Array of rooms"
// @Failure      400 {object} response.Response[any] "Invalid UUID format"
// @Router       /rooms/owner/{ownerId} [get]
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

// ListRoomsByMemberId godoc
// @Summary      List rooms by member
// @Description  Retrieve all rooms a user has joined.
// @Tags         room
// @Produce      json
// @Param        memberId path string true "Member UUID"
// @Success      200 {object} response.Response[[]domain.Room] "Array of rooms"
// @Failure      400 {object} response.Response[any] "Invalid UUID format"
// @Router       /rooms/member/{memberId} [get]
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

// ListUserRooms godoc
// @Summary      List rooms for a user
// @Description  Retrieve all rooms associated with a user — both rooms they own and rooms they've joined. Duplicates are removed.
// @Tags         room
// @Produce      json
// @Param        id path string true "User UUID"
// @Success      200 {object} response.Response[[]domain.Room] "Array of rooms"
// @Failure      400 {object} response.Response[any] "Invalid UUID format"
// @Router       /users/{id}/rooms [get]
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

// UpdateRoom godoc
// @Summary      Update a chat room
// @Description  Update room information by UUID. Only provided fields are updated; omitted fields remain unchanged.
// @Tags         room
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id path string true "Room UUID"
// @Param        request body structs.UpdateRoomRequest true "Fields to update"
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
// @Param        id path string true "Room UUID"
// @Success      200 {object} response.Response[any]
// @Failure      400 {object} response.Response[any] "Invalid UUID format"
// @Failure      500 {object} response.Response[any] "Internal server error"
// @Router       /rooms/{id} [delete]
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

// PopRoom godoc
// @Summary      Pop a chat room
// @Description  Immediately pop a chat room, making it read-only. The authenticated user must be an admin or owner of the room.
// @Tags         room
// @Produce      json
// @Security     BearerAuth
// @Param        id path string true "Room UUID"
// @Success      200 {object} response.Response[any]
// @Failure      400 {object} response.Response[any] "Invalid UUID format"
// @Failure      403 {object} response.Response[any] "Admin permission is required"
// @Failure      410 {object} response.Response[any] "Room has already been popped"
// @Router       /rooms/{id}/pop [post]
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
