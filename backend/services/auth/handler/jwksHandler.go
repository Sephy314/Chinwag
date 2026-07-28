package handler

import (
	"net/http"

	"github.com/Sephy314/chinwag/backend/services/auth/service"
	"github.com/Sephy314/chinwag/backend/services/auth/shared/response"
	"github.com/labstack/echo/v5"
)

type JwksHandler struct {
	service service.JwksServiceInterface
}

func NewJwksHandler(service service.JwksServiceInterface) *JwksHandler {
	return &JwksHandler{
		service: service,
	}
}

func (h *JwksHandler) ServeJWKS(c *echo.Context) error {
	set, err := h.service.GetJwkSet(c.Request().Context())
	if err != nil {
		return err
	}

	return c.JSON(http.StatusOK, response.OK(set))
}
