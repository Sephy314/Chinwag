package handler

import (
	"net/http"
	"time"

	"github.com/Sephy314/chinwag/backend/shared/auth/dpop"
	"github.com/Sephy314/chinwag/backend/services/auth/service"
	"github.com/Sephy314/chinwag/backend/services/auth/shared/errs"
	"github.com/Sephy314/chinwag/backend/services/auth/shared/logger"
	"github.com/Sephy314/chinwag/backend/services/auth/shared/response"
	"github.com/Sephy314/chinwag/backend/services/auth/structs"
	sharedauth "github.com/Sephy314/chinwag/backend/shared/auth"
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
		return c.JSON(errs.ParseError(err))
	}

	return c.JSON(http.StatusOK, response.OK(usr.ToProjection()))
}

func (h *UserHandler) GetUserByID(c *echo.Context) error {
	id := c.Param("id")

	user, err := h.Service.GetUser(c.Request().Context(), id)
	if err != nil {
		return c.JSON(errs.ParseError(err))
	}

	return c.JSON(http.StatusOK, response.OK(user.ToProjection()))
}

func (h *UserHandler) GetUserByEmail(c *echo.Context) error {
	email := c.Param("email")

	user, err := h.Service.GetUserByEmail(c.Request().Context(), email)
	if err != nil {
		return c.JSON(errs.ParseError(err))
	}

	return c.JSON(http.StatusOK, response.OK(user.ToProjection()))
}

func (h *UserHandler) DeleteUser(c *echo.Context) error {
	id := c.Param("id")
	err := h.Service.DeleteUser(c.Request().Context(), id)
	if err != nil {
		return c.JSON(errs.ParseError(err))
	}
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
		return c.JSON(errs.ParseError(err))
	}

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

	return c.JSON(http.StatusOK, response.OK(map[string]interface{}{
		"user": i.ToProjection(),
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
	defer setDPoPNonce(c, nonce)

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
		Path:     "/auth",
		HttpOnly: true,
		Secure:   false,
		SameSite: http.SameSiteLaxMode,
		Expires:  time.Now().Add(time.Hour * 24 * 7),
	})

	h.log.Info("login: success", "email", req.Email)
	return c.JSON(http.StatusOK, response.OK(map[string]string{
		"token": tokens.AccessToken,
	}))
}

func (h *UserHandler) Logout(c *echo.Context) error {
	c.SetCookie(&http.Cookie{
		Name:     "refresh",
		Value:    "",
		Path:     "/auth",
		HttpOnly: true,
		Secure:   false,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
	})

	return c.JSON(http.StatusOK, response.OK[any](nil))
}
