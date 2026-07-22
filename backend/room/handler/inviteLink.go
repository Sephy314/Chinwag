package handler

import (
	"net/http"

	"github.com/Sephy314/chinwag/room/service"
	"github.com/Sephy314/chinwag/room/structs"
	"github.com/Sephy314/chinwag/shared/errs"
	"github.com/Sephy314/chinwag/shared/response"
	"github.com/Sephy314/chinwag/shared/utils"
	"github.com/google/uuid"
	"github.com/labstack/echo/v5"
)

type InviteLinkHandler interface {
	CreateInviteLink(c *echo.Context) error
	JoinByInviteLink(c *echo.Context) error
}

type InviteLinkHandlerImpl struct {
	inviteLinkSvc service.InviteLinkServiceInterface
}

func NewInviteLinkHandler(inviteLinkSvc service.InviteLinkServiceInterface) *InviteLinkHandlerImpl {
	return &InviteLinkHandlerImpl{
		inviteLinkSvc: inviteLinkSvc,
	}
}

// CreateInviteLink godoc
// @Summary      Create an invite link
// @Description  Create a shareable invite link for a room. The authenticated user must have the ADMIN role in the specified room.
// @Tags         invite-link
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        roomId path string true "Room UUID"
// @Param        request body structs.CreateInviteLinkRequest false "TTL configuration"
// @Success      201 {object} response.Response[structs.InviteLinkResponse]
// @Failure      400 {object} response.Response[any] "Invalid request"
// @Failure      403 {object} response.Response[any] "Admin permission is required"
// @Failure      404 {object} response.Response[any] "Room not found"
// @Router       /rooms/{roomId}/invite [post]
func (h *InviteLinkHandlerImpl) CreateInviteLink(c *echo.Context) error {
	roomId, err := uuid.Parse(c.Param("roomId"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, response.Error("invalid room id"))
	}

	userId, err := utils.GetUserIdByEchoContext(c)
	if err != nil {
		return echo.ErrUnauthorized
	}

	uid, err := uuid.Parse(*userId)
	if err != nil {
		return c.JSON(http.StatusBadRequest, response.Error("invalid user id"))
	}

	var req structs.CreateInviteLinkRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, response.Error(err.Error()))
	}

	invite, err := h.inviteLinkSvc.CreateInviteLink(c.Request().Context(), roomId, uid, req)
	if err != nil {
		return c.JSON(errs.ParseError(err))
	}

	return c.JSON(http.StatusCreated, response.Created(invite))
}

// JoinByInviteLink godoc
// @Summary      Join a room via invite link
// @Description  Join a chat room using an invite link token. The user must not be deleted and must not already be a member.
// @Tags         invite-link
// @Produce      json
// @Security     BearerAuth
// @Param        token path string true "Invite link token"
// @Success      200 {object} response.Response[any]
// @Failure      400 {object} response.Response[any] "Invalid token"
// @Failure      404 {object} response.Response[any] "Invite link not found or expired"
// @Failure      409 {object} response.Response[any] "User is already a member"
// @Router       /rooms/invite/{token}/join [post]
func (h *InviteLinkHandlerImpl) JoinByInviteLink(c *echo.Context) error {
	token := c.Param("token")
	if token == "" {
		return c.JSON(http.StatusBadRequest, response.Error("invalid token"))
	}

	userId, err := utils.GetUserIdByEchoContext(c)
	if err != nil {
		return echo.ErrUnauthorized
	}

	uid, err := uuid.Parse(*userId)
	if err != nil {
		return c.JSON(http.StatusBadRequest, response.Error("invalid user id"))
	}

	if err := h.inviteLinkSvc.JoinByInviteLink(c.Request().Context(), token, uid); err != nil {
		return c.JSON(errs.ParseError(err))
	}

	return c.JSON(http.StatusOK, response.OK[any](nil))
}

var _ InviteLinkHandler = (*InviteLinkHandlerImpl)(nil)
