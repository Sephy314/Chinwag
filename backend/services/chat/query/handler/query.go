package handler

import (
	"context"
	"net/http"

	"github.com/Sephy314/chinwag/backend/services/chat/query/service"
	"github.com/Sephy314/chinwag/backend/services/chat/query/structs"
	"github.com/Sephy314/chinwag/backend/services/chat/query/shared/errs"
	"github.com/Sephy314/chinwag/backend/services/chat/query/shared/response"
	"github.com/Sephy314/chinwag/backend/services/chat/query/shared/utils"
	"github.com/google/uuid"
	"github.com/labstack/echo/v5"
)

type QueryHandlerInterface interface {
	Health(c *echo.Context) error
	GetMessage(c *echo.Context) error
	ListMessages(c *echo.Context) error
}

type QueryHandler struct {
	svc service.QueryServiceInterface
}

func NewQueryHandler(svc service.QueryServiceInterface) *QueryHandler {
	return &QueryHandler{svc: svc}
}

func (h *QueryHandler) Health(c *echo.Context) error {
	return c.JSON(http.StatusOK, response.OK[any](nil))
}

func (h *QueryHandler) GetMessage(c *echo.Context) error {
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

	msg, err := h.svc.GetMessage(ctx, messageId, uid)
	if err != nil {
		return c.JSON(errs.ParseError(err))
	}

	return c.JSON(http.StatusOK, response.OK(msg))
}

func (h *QueryHandler) ListMessages(c *echo.Context) error {
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
		After:  c.QueryParam("after"),
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
