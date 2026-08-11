package handler

import (
	"net/http"
	"time"

	"github.com/Sephy314/chinwag/backend/services/auth/service"
	"github.com/Sephy314/chinwag/backend/services/auth/shared/errs"
	"github.com/Sephy314/chinwag/backend/services/auth/shared/logger"
	"github.com/Sephy314/chinwag/backend/services/auth/shared/response"
	"github.com/Sephy314/chinwag/backend/services/auth/structs"
	sharedauth "github.com/Sephy314/chinwag/backend/shared/auth"
	"github.com/Sephy314/chinwag/backend/shared/auth/dpop"
	"github.com/labstack/echo/v5"
)

type UserHandler struct {
	Service *service.UserService
	dpop    service.DPoPServiceInterface
	log     logger.Logger
}

func NewUserHandler(s *service.UserService, log logger.Logger, dpop ...service.DPoPServiceInterface) *UserHandler {
	var dpopSvc service.DPoPServiceInterface
	if len(dpop) > 0 {
		dpopSvc = dpop[0]
	}
	return &UserHandler{
		Service: s,
		dpop:    dpopSvc,
		log:     log,
	}
}

func (h *UserHandler) Health(c *echo.Context) error {
	return c.JSON(http.StatusOK, response.OK[any](nil))
}

func (h *UserHandler) CreateUser(c *echo.Context) error {
	var req structs.CreateUserReq

	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, response.Error(err.Error()))
	}

	usr, err := h.Service.CreateUser(c.Request().Context(), req)
	if err != nil {
		h.log.Warn("user: create failed", "email", req.Email, "error", err)
		return c.JSON(errs.ParseError(err))
	}

	h.log.Info("user: created", "user_id", usr.Id, "email", usr.Email)
	return c.JSON(http.StatusOK, response.OK(usr.ToProjection()))
}

func (h *UserHandler) GetUserByID(c *echo.Context) error {
	id := c.Param("id")

	user, err := h.Service.GetUser(c.Request().Context(), id)
	if err != nil {
		h.log.Debug("user: get by id failed", "user_id", id, "error", err)
		return c.JSON(errs.ParseError(err))
	}

	h.log.Debug("user: get by id", "user_id", id)
	return c.JSON(http.StatusOK, response.OK(user.ToProjection()))
}

func (h *UserHandler) GetUserByEmail(c *echo.Context) error {
	email := c.Param("email")

	user, err := h.Service.GetUserByEmail(c.Request().Context(), email)
	if err != nil {
		h.log.Debug("user: get by email failed", "email", email, "error", err)
		return c.JSON(errs.ParseError(err))
	}

	h.log.Debug("user: get by email", "user_id", user.Id, "email", email)
	return c.JSON(http.StatusOK, response.OK(user.ToProjection()))
}

func (h *UserHandler) DeleteUser(c *echo.Context) error {
	id := c.Param("id")
	err := h.Service.DeleteUser(c.Request().Context(), id)
	if err != nil {
		h.log.Warn("user: delete failed", "user_id", id, "error", err)
		return c.JSON(errs.ParseError(err))
	}
	h.log.Info("user: deleted", "user_id", id)
	return c.JSON(http.StatusOK, response.OK[any](nil))
}

func (h *UserHandler) UpdateUser(c *echo.Context) error {
	id := c.Param("id")

	var req structs.UpdateUserReq
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, response.Error(err.Error()))
	}

	usr, err := h.Service.UpdateUser(c.Request().Context(), id, req)
	if err != nil {
		h.log.Warn("user: update failed", "user_id", id, "error", err)
		return c.JSON(errs.ParseError(err))
	}

	h.log.Info("user: updated", "user_id", usr.Id)
	return c.JSON(http.StatusOK, response.OK(usr.ToProjection()))
}

func (h *UserHandler) WhoAmI(c *echo.Context) error {
	claims, err := sharedauth.ClaimsFromContext(c)
	if err != nil {
		return c.JSON(http.StatusUnauthorized, response.Error("unauthorized"))
	}

	uid := claims.Subject

	i, err := h.Service.GetUser(c.Request().Context(), uid)
	if err != nil {
		return c.JSON(errs.ParseError(err))
	}

	return c.JSON(http.StatusOK, response.OK(map[string]any{
		"user": i.ToProjection(),
		"role": i.Role,
	}))
}

func (h *UserHandler) Login(c *echo.Context) error {
	var req structs.LoginReq
	if err := c.Bind(&req); err != nil {
		h.log.Warn("login: invalid request body", "error", err)
		return c.JSON(http.StatusBadRequest, response.Error(err.Error()))
	}

	proof, nonce, err := h.dpop.Validate(c.Request().Context(), c.Request())
	if err != nil {
		return respondDPoPError(c, nonce, err)
	}
	// Issue the fresh nonce BEFORE writing the response: headers set after
	// c.JSON has committed the response are silently dropped, so the client
	// would keep re-using the just-consumed nonce on the next call.
	setDPoPNonce(c, nonce)

	jkt, err := proof.Thumbprint()
	if err != nil {
		return respondDPoPError(c, nonce, &dpop.Error{Code: dpop.ErrorInvalidProof, Message: "failed to derive DPoP key thumbprint"})
	}

	tokens, err := h.Service.Login(c.Request().Context(), req.Email, req.Password, jkt)
	if err != nil {
		h.log.Warn("login: authentication failed", "email", req.Email, "error", err)
		return c.JSON(errs.ParseError(err))
	}

	c.SetCookie(&http.Cookie{
		Name:     "refresh",
		Value:    tokens.RefreshToken,
		Path:     "/api/auth",
		HttpOnly: true,
		Secure:   false,
		SameSite: http.SameSiteLaxMode,
		Expires:  time.Now().Add(time.Hour * 24 * 7),
	})

	h.log.Info("login: success", "email", req.Email, "user_id", tokens.UserId)
	return c.JSON(http.StatusOK, response.OK(map[string]string{
		"token": tokens.AccessToken,
	}))
}

func (h *UserHandler) Logout(c *echo.Context) error {
	c.SetCookie(&http.Cookie{
		Name:     "refresh",
		Value:    "",
		Path:     "/api/auth",
		HttpOnly: true,
		Secure:   false,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
	})

	h.log.Info("logout")
	return c.JSON(http.StatusOK, response.OK[any](nil))
}
