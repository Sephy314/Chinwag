package handler

import (
	"errors"
	"net/http"
	"time"

	"github.com/Sephy314/chinwag/backend/services/auth/domain"
	"github.com/Sephy314/chinwag/backend/services/auth/service"
	"github.com/Sephy314/chinwag/backend/services/auth/shared/errs"
	"github.com/Sephy314/chinwag/backend/services/auth/shared/response"
	"github.com/Sephy314/chinwag/backend/services/auth/structs"
	"github.com/google/uuid"
	"github.com/labstack/echo/v5"
)

type RefreshHandler interface {
	Refresh(c *echo.Context) error
}

type RefreshHandlerImpl struct {
	service    service.RefreshTokenServiceInterface
	jwtService service.JwtServiceInterface
}

func NewRefreshHandler(service service.RefreshTokenServiceInterface, jwtService service.JwtServiceInterface) *RefreshHandlerImpl {
	return &RefreshHandlerImpl{
		service:    service,
		jwtService: jwtService,
	}
}

func (h *RefreshHandlerImpl) Refresh(c *echo.Context) error {
	ctx := c.Request().Context()

	cookie, err := c.Cookie("refresh")
	if err != nil {
		return c.JSON(http.StatusBadRequest, response.Error("missing refresh token"))
	}

	record, err := h.service.ConsumeRefreshToken(ctx, cookie.Value)
	if err != nil {
		if errors.Is(err, errs.ErrRefreshTokenReused) {
			_ = h.service.RevokeLineage(ctx, record.LineageID)
		}
		return c.JSON(errs.ParseError(err))
	}

	token, err := h.jwtService.NewAccessToken(ctx, record.UserID, domain.USER)
	if err != nil {
		return c.JSON(errs.ParseError(err))
	}

	refreshToken := uuid.Must(uuid.NewV7()).String()

	err = h.service.InsertRefreshToken(ctx, structs.RefreshToken{
		Subject:      record.UserID,
		RefreshToken: refreshToken,
		LineageID:    record.LineageID,
		ParentHash:   service.HashRefreshToken(cookie.Value),
	})
	if err != nil {
		return c.JSON(errs.ParseError(err))
	}

	c.SetCookie(&http.Cookie{
		Name:     "refresh",
		Value:    refreshToken,
		Path:     "/auth",
		Secure:   false,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Expires:  time.Now().Add(time.Hour * 24 * 7),
	})

	return c.JSON(http.StatusOK, response.OK(structs.LoginUserResp{
		Token: *token,
	}))
}
