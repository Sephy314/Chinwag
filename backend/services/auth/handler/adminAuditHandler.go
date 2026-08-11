package handler

import (
	"net/http"

	"github.com/Sephy314/chinwag/backend/services/auth/service"
	"github.com/Sephy314/chinwag/backend/services/auth/shared/errs"
	"github.com/Sephy314/chinwag/backend/services/auth/shared/logger"
	"github.com/Sephy314/chinwag/backend/services/auth/shared/response"
	"github.com/Sephy314/chinwag/backend/services/auth/structs"
	"github.com/labstack/echo/v5"
)

// AdminAuditHandler serves the admin audit log (read-only) and the internal
// system-to-system audit write endpoint used by other services.
type AdminAuditHandler struct {
	audit service.AuditServiceInterface
	log   logger.Logger
}

func NewAdminAuditHandler(audit service.AuditServiceInterface, log logger.Logger) *AdminAuditHandler {
	return &AdminAuditHandler{audit: audit, log: log}
}

func (h *AdminAuditHandler) ListAudit(c *echo.Context) error {
	var req structs.ListAuditRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, response.Error(err.Error()))
	}
	events, meta, err := h.audit.List(c.Request().Context(), req)
	if err != nil {
		return c.JSON(errs.ParseError(err))
	}
	resp := response.OK(events)
	resp.Meta = meta
	return c.JSON(http.StatusOK, resp)
}

// RecordAudit is the internal, unauthenticated endpoint used by other services
// to append audit events. It is not routed by the gateway and is not exposed
// publicly; there is intentionally no update/delete path for audit rows.
func (h *AdminAuditHandler) RecordAudit(c *echo.Context) error {
	var req structs.CreateAuditRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, response.Error(err.Error()))
	}
	if req.AdminId == "" || req.Action == "" || req.TargetType == "" || req.TargetId == "" {
		return c.JSON(http.StatusBadRequest, response.Error("admin_id, action, target_type, target_id are required"))
	}
	if err := h.audit.Record(c.Request().Context(), req.AdminId, req.Action, req.TargetType, req.TargetId, req.Metadata); err != nil {
		return c.JSON(errs.ParseError(err))
	}
	return c.JSON(http.StatusCreated, response.OK[any](nil))
}
