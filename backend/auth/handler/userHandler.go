package handler

import (
	"net/http"

	"github.com/Sephy314/chinwag/auth/errs"
	"github.com/Sephy314/chinwag/auth/service"
	"github.com/Sephy314/chinwag/auth/structs"
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
	id := c.Param("id")

	user, err := h.Service.GetUser(c.Request().Context(), id)
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

		return c.JSON(errs.ParseError(err))
	}

	return c.JSON(http.StatusOK, tokens)
}
