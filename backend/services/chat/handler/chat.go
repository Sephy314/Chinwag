package handler

import (
	"context"
	"net/http"

	"github.com/Sephy314/chinwag/backend/services/chat/service"
	"github.com/Sephy314/chinwag/backend/services/chat/structs"
	"github.com/Sephy314/chinwag/backend/services/chat/shared/errs"
	"github.com/Sephy314/chinwag/backend/services/chat/shared/response"
	"github.com/Sephy314/chinwag/backend/services/chat/shared/utils"
	"github.com/google/uuid"
	"github.com/labstack/echo/v5"
)

type ChatHandlerInterface interface {
	Health(c *echo.Context) error
	CreateMessage(c *echo.Context) error
	GetMessage(c *echo.Context) error
	ListMessages(c *echo.Context) error
	UpdateMessage(c *echo.Context) error
	DeleteMessage(c *echo.Context) error
}

type ChatHandler struct {
	svc service.ChatServiceInterface
}

func NewChatHandler(svc service.ChatServiceInterface) *ChatHandler {
	return &ChatHandler{svc: svc}
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

	ctx := c.Request().Context()
	ctx = context.WithValue(ctx, "authorId", uid)

	msg, err := h.svc.CreateMessage(ctx, roomId, req)
	if err != nil {
		return c.JSON(errs.ParseError(err))
	}

	return c.JSON(http.StatusCreated, response.Created(msg))
}

func (h *ChatHandler) GetMessage(c *echo.Context) error {
	messageId, err := uuid.Parse(c.Param("messageId"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, response.Error("invalid message id"))
	}

	userId, err := utils.GetUserIdByEchoContext(c)
	if err != nil {
		return echo.ErrUnauthorized
	}
	uid, _ := uuid.Parse(*userId)
	ctx := context.WithValue(c.Request().Context(), "userId", uid)

	msg, err := h.svc.GetMessage(ctx, messageId)
	if err != nil {
		return c.JSON(errs.ParseError(err))
	}

	return c.JSON(http.StatusOK, response.OK(msg))
}

func (h *ChatHandler) ListMessages(c *echo.Context) error {
	roomId := c.Param("roomId")
	if roomId == "" {
		return c.JSON(http.StatusBadRequest, response.Error("room id is required"))
	}

	userId, err := utils.GetUserIdByEchoContext(c)
	if err != nil {
		return echo.ErrUnauthorized
	}
	uid, _ := uuid.Parse(*userId)
	ctx := context.WithValue(c.Request().Context(), "userId", uid)

	req := structs.ListMessagesRequest{
		RoomID: roomId,
		Cursor: c.QueryParam("cursor"),
		Limit:  50,
	}

	msgs, meta, err := h.svc.ListMessages(ctx, req)
	if err != nil {
		return c.JSON(errs.ParseError(err))
	}

	var metaResp *structs.CursorMeta
	if meta != nil {
		metaResp = &structs.CursorMeta{
			NextCursor: meta.NextCursor,
			HasMore:    meta.HasMore,
		}
	}

	resp := response.OK(msgs)
	resp.Meta = metaResp
	return c.JSON(http.StatusOK, resp)
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
		return c.JSON(errs.ParseError(err))
	}

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
		return c.JSON(errs.ParseError(err))
	}

	return c.JSON(http.StatusOK, response.OK[any](nil))
}

var _ ChatHandlerInterface = (*ChatHandler)(nil)
