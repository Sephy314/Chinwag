package handler

import (
	"net/http"

	"github.com/Sephy314/chinwag/backend/services/auth/service"
	"github.com/Sephy314/chinwag/backend/services/auth/shared/response"
	"github.com/labstack/echo/v5"
)

const dpopNonceHeader = "DPoP-Nonce"

func setDPoPNonce(c *echo.Context, nonce string) {
	if nonce != "" {
		c.Response().Header().Set(dpopNonceHeader, nonce)
	}
}

func respondDPoPError(c *echo.Context, nonce string, err error) error {
	setDPoPNonce(c, nonce)

	code := service.DPoPErrorInvalid
	msg := "invalid DPoP proof"
	if de, ok := err.(*service.DPoPError); ok {
		code = de.Code
		msg = de.Message
	}

	return c.JSON(http.StatusBadRequest, response.Response[any]{
		Success: false,
		Code:    code,
		Message: msg,
	})
}