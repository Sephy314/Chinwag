package handler

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/Sephy314/chinwag/backend/services/chat/command/service"
	"github.com/Sephy314/chinwag/backend/services/chat/command/shared/errs"
	"github.com/Sephy314/chinwag/backend/services/chat/command/shared/response"
	"github.com/Sephy314/chinwag/backend/services/chat/command/shared/utils"
	"github.com/Sephy314/chinwag/backend/services/chat/command/structs"
	"github.com/google/uuid"
	"github.com/labstack/echo/v5"
)

type ChatHandlerInterface interface {
	Health(c *echo.Context) error
	CreateMessage(c *echo.Context) error
	UpdateMessage(c *echo.Context) error
	DeleteMessage(c *echo.Context) error
}

type ChatHandler struct {
	svc service.ChatServiceInterface
	log *slog.Logger
}

func NewChatHandler(svc service.ChatServiceInterface, log ...*slog.Logger) *ChatHandler {
	l := slog.Default()
	if len(log) > 0 && log[0] != nil {
		l = log[0]
	}
	return &ChatHandler{svc: svc, log: l}
}

func (h *ChatHandler) Health(c *echo.Context) error {
	return c.JSON(http.StatusOK, response.OK[any](nil))
}

func (h *ChatHandler) CreateMessage(c *echo.Context) error {
	roomId, err := uuid.Parse(c.Param("roomId"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, response.Error("invalid room id"))
	}

	userId, err := utils.GetUserIdByEchoContext(c)
	if err != nil {
		return echo.ErrUnauthorized
	}
	uid, _ := uuid.Parse(*userId)

	var req structs.CreateMessageRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, response.Error(err.Error()))
	}
	if req.Id == uuid.Nil {
		return c.JSON(http.StatusBadRequest, response.Error("message id is required"))
	}

	ctx := c.Request().Context()
	ctx = context.WithValue(ctx, "authorId", uid)

	msg, err := h.svc.CreateMessage(ctx, roomId, req)
	if err != nil {
		h.log.Warn("chat: message create failed", "room_id", roomId.String(), "user_id", uid.String(), "message_id", req.Id.String(), "error", err)
		return c.JSON(errs.ParseError(err))
	}

	h.log.Info("chat: message created", "room_id", roomId.String(), "user_id", uid.String(), "message_id", msg.Id)
	return c.JSON(http.StatusCreated, response.Created(msg))
}

func (h *ChatHandler) UpdateMessage(c *echo.Context) error {
	messageId, err := uuid.Parse(c.Param("messageId"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, response.Error("invalid message id"))
	}

	userId, err := utils.GetUserIdByEchoContext(c)
	if err != nil {
		return echo.ErrUnauthorized
	}
	uid, _ := uuid.Parse(*userId)

	var req structs.UpdateMessageRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, response.Error(err.Error()))
	}

	msg, err := h.svc.UpdateMessage(c.Request().Context(), messageId, uid, req)
	if err != nil {
		h.log.Warn("chat: message update failed", "message_id", messageId.String(), "user_id", uid.String(), "error", err)
		return c.JSON(errs.ParseError(err))
	}

	h.log.Info("chat: message updated", "message_id", messageId.String(), "user_id", uid.String())
	return c.JSON(http.StatusOK, response.OK(msg))
}

func (h *ChatHandler) DeleteMessage(c *echo.Context) error {
	messageId, err := uuid.Parse(c.Param("messageId"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, response.Error("invalid message id"))
	}

	userId, err := utils.GetUserIdByEchoContext(c)
	if err != nil {
		return echo.ErrUnauthorized
	}
	uid, _ := uuid.Parse(*userId)

	if err := h.svc.DeleteMessage(c.Request().Context(), messageId, uid); err != nil {
		h.log.Warn("chat: message delete failed", "message_id", messageId.String(), "user_id", uid.String(), "error", err)
		return c.JSON(errs.ParseError(err))
	}

	h.log.Info("chat: message deleted", "message_id", messageId.String(), "user_id", uid.String())
	return c.JSON(http.StatusOK, response.OK[any](nil))
}

var _ ChatHandlerInterface = (*ChatHandler)(nil)
