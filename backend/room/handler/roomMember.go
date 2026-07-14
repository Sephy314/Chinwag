package handler

import (
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

type RoomMemberHandler interface {
	AddMember(c *echo.Context) error
	RemoveMember(c *echo.Context) error
	ListMembers(c *echo.Context) error
	GetMember(c *echo.Context) error
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

// AddMember godoc
// @Summary      Add a room member
// @Description  Add a user to a chat room. The authenticated user must have the ADMIN role in the specified room. If role is omitted, the user is added as MEMBER.
// @Tags         room-member
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        roomId path string true "Room UUID" example(550e8400-e29b-41d4-a716-446655440000)
// @Param        request body object true "Member to add" example({"userId":"660e8400-e29b-41d4-a716-446655440000","role":0})
// @Success      201 {object} response.Response[any]
// @Failure      400 {object} response.Response[any] "Invalid request body or UUID format"
// @Failure      403 {object} response.Response[any] "Admin permission is required"
// @Failure      404 {object} response.Response[any] "Room or user not found"
// @Failure      409 {object} response.Response[any] "User is already a member of the room"
// @Router       /rooms/{roomId}/members [post]
func (h *RoomMemberHandlerImpl) AddMember(c *echo.Context) error {
	roomId, err := uuid.Parse(c.Param("roomId"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, response.Error( "invalid room id"))
	}

	var body struct {
		UserId uuid.UUID    `json:"userId"`
		Role   *domain.Role `json:"role"`
	}
	if err := c.Bind(&body); err != nil {
		return c.JSON(http.StatusBadRequest, response.Error( err.Error()))
	}

	ok, err := utils.IsManager(c, roomId, h.service)
	if err != nil {
		return c.JSON(errs.ParseError(err))
	}

	if !ok {
		return c.JSON(http.StatusForbidden, response.Error("Admin permission is required"))
	}

	req := structs.RoomUser{
		UserId: body.UserId,
		RoomId: roomId,
		Role:   body.Role,
	}

	if err := h.service.InviteUser(c.Request().Context(), req); err != nil {
		return c.JSON(errs.ParseError(err))
	}

	return c.JSON(http.StatusCreated, response.OK[any](nil))
}

// RemoveMember godoc
// @Summary      Remove a room member
// @Description  Remove a user from a chat room. The authenticated user must have the ADMIN role in the specified room.
// @Tags         room-member
// @Produce      json
// @Security     BearerAuth
// @Param        roomId path string true "Room UUID" example(550e8400-e29b-41d4-a716-446655440000)
// @Param        userId path string true "User UUID to remove" example(660e8400-e29b-41d4-a716-446655440000)
// @Success      200 {object} response.Response[any]
// @Failure      400 {object} response.Response[any] "Invalid UUID format"
// @Failure      404 {object} response.Response[any] "Room, user, or membership not found"
// @Router       /rooms/{roomId}/members/{userId} [delete]
func (h *RoomMemberHandlerImpl) RemoveMember(c *echo.Context) error {
	roomId, err := uuid.Parse(c.Param("roomId"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, response.Error( "invalid room id"))
	}

	userId, err := uuid.Parse(c.Param("userId"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, response.Error( "invalid user id"))
	}

	req := structs.RoomUser{
		UserId: userId,
		RoomId: roomId,
	}

	if err := h.service.KickUser(c.Request().Context(), req); err != nil {
		return c.JSON(errs.ParseError(err))
	}

	return c.JSON(http.StatusOK, response.OK[any](nil))
}

// ListMembers godoc
// @Summary      List room members
// @Description  Retrieve all members of a chat room.
// @Tags         room-member
// @Produce      json
// @Security     BearerAuth
// @Param        roomId path string true "Room UUID" example(550e8400-e29b-41d4-a716-446655440000)
// @Success      200 {object} response.Response[[]domain.RoomMember] "Array of room members"
// @Failure      400 {object} response.Response[any] "Invalid UUID format"
// @Router       /rooms/{roomId}/members [get]
func (h *RoomMemberHandlerImpl) ListMembers(c *echo.Context) error {
	roomId, err := uuid.Parse(c.Param("roomId"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, response.Error( "invalid room id"))
	}

	members, err := h.service.GetUserByRoomId(c.Request().Context(), roomId)
	if err != nil {
		return c.JSON(errs.ParseError(err))
	}

	return c.JSON(http.StatusOK, response.OK(members))
}

// GetMember godoc
// @Summary      Get a room member
// @Description  Retrieve a specific user's membership info in a specific chat room.
// @Tags         room-member
// @Produce      json
// @Security     BearerAuth
// @Param        roomId path string true "Room UUID" example(550e8400-e29b-41d4-a716-446655440000)
// @Param        userId path string true "User UUID" example(660e8400-e29b-41d4-a716-446655440000)
// @Success      200 {object} response.Response[domain.RoomMember] "Membership info found"
// @Failure      400 {object} response.Response[any] "Invalid UUID format"
// @Failure      404 {object} response.Response[any] "Membership not found"
// @Router       /rooms/{roomId}/members/{userId} [get]
func (h *RoomMemberHandlerImpl) GetMember(c *echo.Context) error {
	roomId, err := uuid.Parse(c.Param("roomId"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, response.Error( "invalid room id"))
	}

	userId, err := uuid.Parse(c.Param("userId"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, response.Error( "invalid user id"))
	}

	member, err := h.service.GetUserByRoomIdAndUserId(c.Request().Context(), userId, roomId)
	if err != nil {
		return c.JSON(errs.ParseError(err))
	}

	return c.JSON(http.StatusOK, response.OK(member))
}

var _ RoomMemberHandler = (*RoomMemberHandlerImpl)(nil)
