package utils

import (
	sharedauth "github.com/Sephy314/chinwag/backend/shared/auth"
	"github.com/labstack/echo/v5"
)

func GetUserIdByEchoContext(ctx *echo.Context) (*string, error) {
	claims, err := sharedauth.ClaimsFromContext(ctx)
	if err != nil {
		return nil, err
	}

	return &claims.Subject, nil
}
