package utils

import (
	"fmt"

	"github.com/golang-jwt/jwt/v5"
	"github.com/labstack/echo/v5"
)

func GetUserIdByEchoContext(c *echo.Context) (*string, error) {
	token, ok := c.Get("user").(*jwt.Token)
	if !ok {
		return nil, fmt.Errorf("user not found in context")
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return nil, fmt.Errorf("invalid claims")
	}

	sub, ok := claims["sub"].(string)
	if !ok {
		return nil, fmt.Errorf("sub claim not found")
	}

	return &sub, nil
}
