package handler

import (
	"log/slog"
	"net/http"

	"github.com/Sephy314/chinwag/backend/services/chat/query/service"
	"github.com/Sephy314/chinwag/backend/services/chat/query/shared/errs"
	"github.com/Sephy314/chinwag/backend/services/chat/query/shared/response"
	"github.com/Sephy314/chinwag/backend/services/chat/query/structs"
	"github.com/google/uuid"
	"github.com/labstack/echo/v5"
)

type AdminQueryHandler struct {
	svc service.QueryServiceInterface
	log *slog.Logger
}

func NewAdminQueryHandler(svc service.QueryServiceInterface, log *slog.Logger) *AdminQueryHandler {
	return &AdminQueryHandler{svc: svc, log: log}
}

func (h *AdminQueryHandler) ListMessages(c *echo.Context) error {
	var req structs.AdminListMessagesRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, response.Error(err.Error()))
	}
	msgs, meta, err := h.svc.AdminListMessages(c.Request().Context(), req)
	if err != nil {
		h.log.Warn("admin chat: list messages failed", "error", err)
		return c.JSON(errs.ParseError(err))
	}

	var metaResp *structs.CursorMeta
	if meta != nil {
		metaResp = &structs.CursorMeta{NextCursor: meta.NextCursor, HasMore: meta.HasMore}
	}
	resp := response.OK(msgs)
	resp.Meta = metaResp
	return c.JSON(http.StatusOK, resp)
}

func (h *AdminQueryHandler) GetMessage(c *echo.Context) error {
	messageId, err := uuid.Parse(c.Param("messageId"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, response.Error("invalid message id"))
	}
	msg, err := h.svc.AdminGetMessage(c.Request().Context(), messageId)
	if err != nil {
		h.log.Warn("admin chat: get message failed", "message_id", messageId.String(), "error", err)
		return c.JSON(errs.ParseError(err))
	}
	return c.JSON(http.StatusOK, response.OK(msg))
}

func (h *AdminQueryHandler) StatsMessages(c *echo.Context) error {
	n, err := h.svc.AdminCountMessages(c.Request().Context())
	if err != nil {
		return c.JSON(errs.ParseError(err))
	}
	return c.JSON(http.StatusOK, response.OK(map[string]int64{"total_messages": n}))
}
