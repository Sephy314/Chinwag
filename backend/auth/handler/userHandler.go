package handler

import (
	"net/http"
	"time"

	"github.com/Sephy314/chinwag/auth/domain"
	"github.com/Sephy314/chinwag/auth/service"
	"github.com/Sephy314/chinwag/auth/structs"
	"github.com/Sephy314/chinwag/shared/errs"
	"github.com/Sephy314/chinwag/shared/utils"
	"github.com/golang-jwt/jwt/v5"
	"github.com/labstack/echo/v5"
)

type UserHandler struct {
	Service *service.UserService
}

func NewUserHandler(s *service.UserService) *UserHandler {
	hdl := UserHandler{
		Service: s,
	}
	return &hdl
}

func (h *UserHandler) Health(c *echo.Context) error {
	return c.JSON(http.StatusOK, "")
}

func (h *UserHandler) CreateUser(c *echo.Context) error {
	var req structs.CreateUserReq

	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, err.Error())
	}

	usr, err := h.Service.CreateUser(c.Request().Context(), req)

	if err != nil {
		return c.JSON(errs.ParseError(err))
	}

	usrProjection := usr.ToProjection()

	return c.JSON(http.StatusOK, usrProjection)
}

func (h *UserHandler) GetUser(c *echo.Context) error {
	var user *domain.User
	var err error

	id := c.Param("id")

	if utils.IsEmail(id) {
		user, err = h.Service.GetUserByEmail(c.Request().Context(), id)
	} else {
		user, err = h.Service.GetUser(c.Request().Context(), id)
	}

	if err != nil {
		return c.JSON(errs.ParseError(err))
	}

	return c.JSON(http.StatusOK, user.ToProjection())
}

func (h *UserHandler) DeleteUser(c *echo.Context) error {
	id := c.Param("id")
	err := h.Service.DeleteUser(c.Request().Context(), id)
	if err != nil {
		return c.JSON(errs.ParseError(err))
	}
	return c.JSON(http.StatusOK, "ok")
}

func (h *UserHandler) Login(c *echo.Context) error {
	var req structs.LoginReq
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, err.Error())
	}

	tokens, err := h.Service.Login(c.Request().Context(), req.Email, req.Password)
	if err != nil {
		status, res := errs.ParseError(err)

		return c.JSON(status, res)
	}

	c.SetCookie(&http.Cookie{
		Name:     "refresh",
		Value:    tokens.RefreshToken,
		Path:     "/auth",
		Secure:   true,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Expires:  time.Now().Add(time.Hour * 24 * 7),
	})

	return c.JSON(http.StatusOK, map[string]interface{}{
		"token": tokens.AccessToken,
	})
}

func (h *UserHandler) WhoAmI(c *echo.Context) error {
	token, err := echo.ContextGet[*jwt.Token](c, "user")
	if err != nil {
		return c.JSON(errs.ParseError(err))
	}

	uid, err := token.Claims.GetSubject()
	if err != nil {
		return c.JSON(errs.ParseError(err))
	}

	i, err := h.Service.GetUser(c.Request().Context(), uid)
	if err != nil {
		return c.JSON(errs.ParseError(err))
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"user": i.ToProjection(),
	})
}
