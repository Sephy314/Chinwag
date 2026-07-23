package handler

import (
	"net/http"

	"github.com/Sephy314/chinwag/auth/service"
	"github.com/Sephy314/chinwag/shared/response"
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

// ServeJWKS godoc
// @Summary      Serve JWKS
// @Description  Return the JSON Web Key Set (JWKS) containing the public keys used to verify JWT tokens issued by this service. The set includes active and inactive (rotated but still valid) ECDSA P-256 keys identified by their kid (key ID).
// @Tags         auth
// @Produce      json
// @Success      200 {object} map[string]interface{}
// @Failure      500 {object} map[string]interface{}
// @Router       /auth/.well-known/jwks.json [get]
func (h *JwksHandler) ServeJWKS(c *echo.Context) error {
	set, err := h.service.GetJwkSet(c.Request().Context())
	if err != nil {
		return err
	}

	return c.JSON(http.StatusOK, response.OK(set))
}
