package auth

import (
	"github.com/labstack/echo/v5"
)

func ClaimsFromContext(c *echo.Context) (*Claims, error) {
	val := c.Get(ClaimsContextKey)
	if val == nil {
		return nil, echo.ErrUnauthorized
	}
	claims, ok := val.(*Claims)
	if !ok {
		return nil, echo.ErrUnauthorized
	}
	return claims, nil
}

func UserIDFromContext(c *echo.Context) (string, error) {
	claims, err := ClaimsFromContext(c)
	if err != nil {
		return "", err
	}
	return claims.Subject, nil
}
