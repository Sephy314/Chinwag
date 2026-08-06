package handler

import (
	"net/http"

	"github.com/Sephy314/chinwag/backend/shared/auth/dpop"
	"github.com/Sephy314/chinwag/backend/services/auth/shared/response"
	"github.com/labstack/echo/v5"
)

func setDPoPNonce(c *echo.Context, nonce string) {
	if nonce != "" {
		c.Response().Header().Set(dpop.NonceHeader, nonce)
	}
}

func respondDPoPError(c *echo.Context, nonce string, err error) error {
	setDPoPNonce(c, nonce)

	code := dpop.ErrorInvalidProof
	msg := "invalid DPoP proof"
	status := http.StatusBadRequest
	if de, ok := err.(*dpop.Error); ok {
		code = de.Code
		msg = de.Message
		if de.Status != 0 {
			status = de.Status
		}
	}

	return c.JSON(status, response.Response[any]{
		Success: false,
		Code:    code,
		Message: msg,
	})
}