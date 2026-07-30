package utils

import (
	"context"
	"regexp"

	"github.com/google/uuid"
	"github.com/labstack/echo/v5"
	sharedauth "github.com/Sephy314/chinwag/backend/shared/auth"
)

func IsEmail(email string) bool {
	var emailRegex = regexp.MustCompile(`^[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Za-z]{2,}$`)

	return emailRegex.MatchString(email)
}

func GetUserIdByEchoContext(ctx *echo.Context) (*string, error) {
	claims, err := sharedauth.ClaimsFromContext(ctx)
	if err != nil {
		return nil, err
	}

	return &claims.Subject, nil
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
