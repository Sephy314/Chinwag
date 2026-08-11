package handler

import (
	"net/http"

	"github.com/Sephy314/chinwag/backend/services/auth/service"
	"github.com/Sephy314/chinwag/backend/services/auth/shared/errs"
	"github.com/Sephy314/chinwag/backend/services/auth/shared/logger"
	"github.com/Sephy314/chinwag/backend/services/auth/shared/response"
	"github.com/Sephy314/chinwag/backend/services/auth/structs"
	sharedauth "github.com/Sephy314/chinwag/backend/shared/auth"
	"github.com/labstack/echo/v5"
)

type AdminUserHandler struct {
	users    *service.UserService
	sessions *service.RefreshTokenService
	audit    service.AuditServiceInterface
	log      logger.Logger
}

func NewAdminUserHandler(users *service.UserService, sessions *service.RefreshTokenService, audit service.AuditServiceInterface, log logger.Logger) *AdminUserHandler {
	return &AdminUserHandler{users: users, sessions: sessions, audit: audit, log: log}
}

func adminID(c *echo.Context) string {
	claims, err := sharedauth.ClaimsFromContext(c)
	if err != nil {
		return ""
	}
	return claims.Subject
}

func (h *AdminUserHandler) ListUsers(c *echo.Context) error {
	var req structs.ListUsersRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, response.Error(err.Error()))
	}
	users, meta, err := h.users.AdminListUsers(c.Request().Context(), req)
	if err != nil {
		return c.JSON(errs.ParseError(err))
	}
	resp := response.OK(users)
	resp.Meta = meta
	return c.JSON(http.StatusOK, resp)
}

func (h *AdminUserHandler) GetUser(c *echo.Context) error {
	user, err := h.users.AdminGetUser(c.Request().Context(), c.Param("id"))
	if err != nil {
		return c.JSON(errs.ParseError(err))
	}
	return c.JSON(http.StatusOK, response.OK(user))
}

func (h *AdminUserHandler) CreateUser(c *echo.Context) error {
	var req structs.CreateAdminUserRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, response.Error(err.Error()))
	}
	user, err := h.users.AdminCreateUser(c.Request().Context(), req)
	if err != nil {
		return c.JSON(errs.ParseError(err))
	}
	_ = h.audit.Record(c.Request().Context(), adminID(c), "user.create", "user", user.Id,
		map[string]any{"email": user.Email, "role": user.Role})
	return c.JSON(http.StatusCreated, response.Created(user))
}

func (h *AdminUserHandler) UpdateUser(c *echo.Context) error {
	id := c.Param("id")
	var req structs.UpdateAdminUserRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, response.Error(err.Error()))
	}
	user, err := h.users.AdminUpdateUser(c.Request().Context(), id, req)
	if err != nil {
		return c.JSON(errs.ParseError(err))
	}
	_ = h.audit.Record(c.Request().Context(), adminID(c), "user.update", "user", id, nil)
	return c.JSON(http.StatusOK, response.OK(user))
}

func (h *AdminUserHandler) UpdateRole(c *echo.Context) error {
	id := c.Param("id")
	var req structs.UpdateUserRoleRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, response.Error(err.Error()))
	}
	if err := h.users.AdminSetRole(c.Request().Context(), id, adminID(c), req.Role); err != nil {
		return c.JSON(errs.ParseError(err))
	}
	user, err := h.users.AdminGetUser(c.Request().Context(), id)
	if err != nil {
		return c.JSON(errs.ParseError(err))
	}
	_ = h.audit.Record(c.Request().Context(), adminID(c), "user.role_change", "user", id,
		map[string]any{"role": req.Role})
	return c.JSON(http.StatusOK, response.OK(user))
}

func (h *AdminUserHandler) DisableUser(c *echo.Context) error {
	id := c.Param("id")
	if err := h.users.DeleteUser(c.Request().Context(), id); err != nil {
		return c.JSON(errs.ParseError(err))
	}
	_ = h.audit.Record(c.Request().Context(), adminID(c), "user.disable", "user", id, nil)
	return c.JSON(http.StatusOK, response.OK[any](nil))
}

func (h *AdminUserHandler) RestoreUser(c *echo.Context) error {
	id := c.Param("id")
	user, err := h.users.AdminRestoreUser(c.Request().Context(), id)
	if err != nil {
		return c.JSON(errs.ParseError(err))
	}
	_ = h.audit.Record(c.Request().Context(), adminID(c), "user.restore", "user", id, nil)
	return c.JSON(http.StatusOK, response.OK(user))
}

func (h *AdminUserHandler) ListUserSessions(c *echo.Context) error {
	id := c.Param("id")
	sessions, err := h.sessions.ListSessionsByUser(c.Request().Context(), id)
	if err != nil {
		return c.JSON(errs.ParseError(err))
	}
	return c.JSON(http.StatusOK, response.OK(sessions))
}

func (h *AdminUserHandler) RevokeUserSessions(c *echo.Context) error {
	id := c.Param("id")
	if err := h.sessions.RevokeUserSessions(c.Request().Context(), id); err != nil {
		return c.JSON(errs.ParseError(err))
	}
	_ = h.audit.Record(c.Request().Context(), adminID(c), "session.revoke_all", "user", id, nil)
	return c.JSON(http.StatusOK, response.OK[any](nil))
}

func (h *AdminUserHandler) StatsUsers(c *echo.Context) error {
	n, err := h.users.CountUsers(c.Request().Context())
	if err != nil {
		return c.JSON(errs.ParseError(err))
	}
	return c.JSON(http.StatusOK, response.OK(structs.StatsResponse{Count: n}))
}
