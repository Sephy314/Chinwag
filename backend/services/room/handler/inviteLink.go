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

type InviteLinkHandler interface {
	CreateInviteLink(c *echo.Context) error
	JoinByInviteLink(c *echo.Context) error
}

type InviteLinkHandlerImpl struct {
	inviteLinkSvc service.InviteLinkServiceInterface
	log           *slog.Logger
}

func NewInviteLinkHandler(inviteLinkSvc service.InviteLinkServiceInterface, log ...*slog.Logger) *InviteLinkHandlerImpl {
	l := slog.Default()
	if len(log) > 0 && log[0] != nil {
		l = log[0]
	}
	return &InviteLinkHandlerImpl{
		inviteLinkSvc: inviteLinkSvc,
		log:           l,
	}
}

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
		h.log.Warn("room: invite link create failed", "room_id", roomId.String(), "user_id", uid.String(), "error", err)
		return c.JSON(errs.ParseError(err))
	}

	h.log.Info("room: invite link created", "room_id", roomId.String(), "user_id", uid.String())
	return c.JSON(http.StatusCreated, response.Created(invite))
}

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

	roomId, err := h.inviteLinkSvc.JoinByInviteLink(c.Request().Context(), token, uid)
	if err != nil {
		h.log.Warn("room: invite join failed", "user_id", uid.String(), "token", token, "error", err)
		return c.JSON(errs.ParseError(err))
	}

	h.log.Info("room: invite join", "room_id", roomId.String(), "user_id", uid.String())
	return c.JSON(http.StatusOK, response.OK(map[string]string{"room_id": roomId.String()}))
}

var _ InviteLinkHandler = (*InviteLinkHandlerImpl)(nil)
