package handler

import (
	"errors"
	"net/http"
	"time"

	"github.com/Sephy314/chinwag/backend/shared/auth/dpop"
	"github.com/Sephy314/chinwag/backend/services/auth/domain"
	"github.com/Sephy314/chinwag/backend/services/auth/service"
	"github.com/Sephy314/chinwag/backend/services/auth/shared/cache"
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
	locker     cache.Cache
	dpop       service.DPoPServiceInterface
}

func NewRefreshHandler(service service.RefreshTokenServiceInterface, jwtService service.JwtServiceInterface, locker cache.Cache, dpop service.DPoPServiceInterface) *RefreshHandlerImpl {
	return &RefreshHandlerImpl{
		service:    service,
		jwtService: jwtService,
		locker:     locker,
		dpop:       dpop,
	}
}

func (h *RefreshHandlerImpl) Refresh(c *echo.Context) error {
	ctx := c.Request().Context()

	proof, nonce, err := h.dpop.Validate(ctx, c.Request())
	if err != nil {
		return respondDPoPError(c, nonce, err)
	}
	defer setDPoPNonce(c, nonce)

	jkt, err := proof.Thumbprint()
	if err != nil {
		return respondDPoPError(c, nonce, &dpop.Error{Code: dpop.ErrorInvalidProof, Message: "failed to derive DPoP key thumbprint"})
	}

	cookie, err := c.Cookie("refresh")
	if err != nil {
		return c.JSON(http.StatusBadRequest, response.Error("missing refresh token"))
	}

	record, err := h.service.GetRefreshToken(ctx, cookie.Value)
	if err != nil {
		return c.JSON(errs.ParseError(err))
	}
	if record.Jkt != "" && record.Jkt != jkt {
		return respondDPoPError(c, nonce, &dpop.Error{Code: dpop.ErrorInvalidProof, Message: "DPoP key does not match the bound refresh token"})
	}

	lockKey := "refresh:lock:" + service.HashRefreshToken(cookie.Value)
	lockToken := uuid.Must(uuid.NewV7()).String()
	acquired, err := h.locker.AcquireLock(ctx, lockKey, lockToken, time.Second*5)
	if err != nil {
		return c.JSON(errs.ParseError(err))
	}
	if !acquired {
		return c.JSON(http.StatusTooManyRequests, response.Error("refresh in progress"))
	}
	defer func() {
		_ = h.locker.ReleaseLock(ctx, lockKey, lockToken)
	}()

	consumed, err := h.service.ConsumeRefreshToken(ctx, cookie.Value)
	if err != nil {
		if errors.Is(err, errs.ErrRefreshTokenReused) {
			_ = h.service.RevokeLineage(ctx, consumed.LineageID)
		}
		return c.JSON(errs.ParseError(err))
	}

	token, err := h.jwtService.NewAccessToken(ctx, consumed.UserID, domain.USER, jkt)
	if err != nil {
		return c.JSON(errs.ParseError(err))
	}

	refreshToken := uuid.Must(uuid.NewV7()).String()

	err = h.service.InsertRefreshToken(ctx, structs.RefreshToken{
		Subject:      consumed.UserID,
		RefreshToken: refreshToken,
		LineageID:    consumed.LineageID,
		ParentHash:   service.HashRefreshToken(cookie.Value),
		Jkt:          jkt,
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
