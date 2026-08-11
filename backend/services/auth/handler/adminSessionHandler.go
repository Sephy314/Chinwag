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

type AdminSessionHandler struct {
	sessions *service.RefreshTokenService
	audit    service.AuditServiceInterface
	log      logger.Logger
}

func NewAdminSessionHandler(sessions *service.RefreshTokenService, audit service.AuditServiceInterface, log logger.Logger) *AdminSessionHandler {
	return &AdminSessionHandler{sessions: sessions, audit: audit, log: log}
}

// ListSessions returns all sessions, newest first. Sessions are aggregated in
// Redis, so pagination is applied in-memory over the sorted lineage list using
// the opaque `cursor` (a lineage id) + `limit`.
func (h *AdminSessionHandler) ListSessions(c *echo.Context) error {
	var req structs.ListSessionsRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, response.Error(err.Error()))
	}
	limit := req.Limit
	if limit <= 0 || limit > 200 {
		limit = 50
	}

	all, err := h.sessions.ListAllSessions(c.Request().Context())
	if err != nil {
		return c.JSON(errs.ParseError(err))
	}

	start := 0
	if req.Cursor != "" {
		for i, s := range all {
			if s.LineageId == req.Cursor {
				start = i + 1
				break
			}
		}
	}

	end := start + limit
	hasMore := end < len(all)
	if end > len(all) {
		end = len(all)
	}
	page := all[start:end]

	var meta *structs.CursorMeta
	if hasMore && len(page) > 0 {
		meta = &structs.CursorMeta{
			NextCursor: page[len(page)-1].LineageId,
			HasMore:    true,
		}
	}
	resp := response.OK(page)
	resp.Meta = meta
	return c.JSON(http.StatusOK, resp)
}

func (h *AdminSessionHandler) GetSession(c *echo.Context) error {
	s, err := h.sessions.GetLineage(c.Request().Context(), c.Param("id"))
	if err != nil {
		return c.JSON(errs.ParseError(err))
	}
	if s == nil {
		return c.JSON(http.StatusNotFound, response.Error("session not found"))
	}
	return c.JSON(http.StatusOK, response.OK(s))
}

func (h *AdminSessionHandler) RevokeSession(c *echo.Context) error {
	id := c.Param("id")
	if err := h.sessions.RevokeLineage(c.Request().Context(), id); err != nil {
		return c.JSON(errs.ParseError(err))
	}
	_ = h.audit.Record(c.Request().Context(), adminID(c), "session.revoke", "session", id, nil)
	return c.JSON(http.StatusOK, response.OK[any](nil))
}

func (h *AdminSessionHandler) StatsSessions(c *echo.Context) error {
	n, err := h.sessions.CountSessions(c.Request().Context())
	if err != nil {
		return c.JSON(errs.ParseError(err))
	}
	return c.JSON(http.StatusOK, response.OK(structs.StatsResponse{Count: n}))
}
