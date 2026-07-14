package handler

import (
	"net/http"

	"github.com/Sephy314/chinwag/auth/service"
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
// @Success      200 {object} map[string]interface{} "JWKS set with keys array (each key has kid, kty, crv, x, y fields)"
// @Failure      500 {string} string "Internal server error"
// @Router       /auth/.well-known/jwks.json [get]
func (h *JwksHandler) ServeJWKS(c *echo.Context) error {
	set, err := h.service.GetJwkSet(c.Request().Context())
	if err != nil {
		return err
	}

	return c.JSON(http.StatusOK, set)
}
