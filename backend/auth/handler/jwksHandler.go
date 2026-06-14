package handler

import (
	"net/http"

	"github.com/Sephy314/chinwag/auth/service"
	"github.com/labstack/echo/v5"
)

type JwksHandler struct {
	service service.JwksService
}

func NewJwksHandler(service service.JwksService) *JwksHandler {
	return &JwksHandler{
		service: service,
	}
}

func (h *JwksHandler) ServeJWKS(c *echo.Context) error {
	set, err := h.service.GetJwkSet(c.Request().Context())
	if err != nil {
		return err
	}

	return c.JSON(http.StatusOK, set)
}
