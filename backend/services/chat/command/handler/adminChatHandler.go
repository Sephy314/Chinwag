package handler

import (
	"log/slog"
	"net/http"

	"github.com/Sephy314/chinwag/backend/services/chat/command/service"
	"github.com/Sephy314/chinwag/backend/services/chat/command/shared/errs"
	"github.com/Sephy314/chinwag/backend/services/chat/command/shared/response"
	sharedauth "github.com/Sephy314/chinwag/backend/shared/auth"
	"github.com/google/uuid"
	"github.com/labstack/echo/v5"
)

type AdminChatHandler struct {
	svc   service.ChatServiceInterface
	audit *service.AuditClient
	log   *slog.Logger
}

func NewAdminChatHandler(svc service.ChatServiceInterface, audit *service.AuditClient, log *slog.Logger) *AdminChatHandler {
	return &AdminChatHandler{svc: svc, audit: audit, log: log}
}

func (h *AdminChatHandler) adminID(c *echo.Context) string {
	claims, err := sharedauth.ClaimsFromContext(c)
	if err != nil {
		return ""
	}
	return claims.Subject
}

// DeleteMessage soft-deletes any message (bypasses author + popped-room guards).
func (h *AdminChatHandler) DeleteMessage(c *echo.Context) error {
	messageId, err := uuid.Parse(c.Param("messageId"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, response.Error("invalid message id"))
	}

	if err := h.svc.AdminDeleteMessage(c.Request().Context(), messageId); err != nil {
		h.log.Warn("admin chat: message delete failed", "message_id", messageId.String(), "error", err)
		return c.JSON(errs.ParseError(err))
	}

	h.audit.Record(c.Request().Context(), h.adminID(c), "message.delete", "message", messageId.String(), nil)
	h.log.Info("admin chat: message deleted", "message_id", messageId.String(), "admin_id", h.adminID(c))
	return c.JSON(http.StatusOK, response.OK[any](nil))
}
