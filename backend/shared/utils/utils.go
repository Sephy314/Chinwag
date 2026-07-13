package utils

import (
	"context"
	"regexp"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/labstack/echo/v5"
)

func IsEmail(email string) bool {
	var emailRegex = regexp.MustCompile(`^[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Za-z]{2,}$`)

	return emailRegex.MatchString(email)
}

func GetUserIdByEchoContext(ctx *echo.Context) (*string, error) {
	token, err := echo.ContextGet[*jwt.Token](ctx, "user")

	if err != nil {
		return nil, err
	}

	uid, err := token.Claims.GetSubject()

	return &uid, err
}

type ManagerChecker interface {
	HasManagerPermission(ctx context.Context, userID, roomID uuid.UUID) (bool, error)
}

func IsManager(
	c *echo.Context,
	roomID uuid.UUID,
	checker ManagerChecker,
) (bool, error) {
	userID, err := GetUserIdByEchoContext(c)
	if err != nil {
		return false, err
	}

	uid, err := uuid.Parse(*userID)

	if err != nil {
		return false, err
	}

	return checker.HasManagerPermission(c.Request().Context(), uid, roomID)
}
