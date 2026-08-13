package handler

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/Sephy314/chinwag/backend/services/auth/service"
	"github.com/Sephy314/chinwag/backend/services/auth/shared/cache"
	"github.com/Sephy314/chinwag/backend/services/auth/shared/errs"
	"github.com/Sephy314/chinwag/backend/services/auth/shared/logger"
	"github.com/Sephy314/chinwag/backend/services/auth/shared/response"
	"github.com/Sephy314/chinwag/backend/services/auth/structs"
	"github.com/Sephy314/chinwag/backend/shared/auth/dpop"
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
	users      *service.UserService
	log        logger.Logger
}

func NewRefreshHandler(service service.RefreshTokenServiceInterface, jwtService service.JwtServiceInterface, locker cache.Cache, dpop service.DPoPServiceInterface, users *service.UserService, log logger.Logger) *RefreshHandlerImpl {
	return &RefreshHandlerImpl{
		service:    service,
		jwtService: jwtService,
		locker:     locker,
		dpop:       dpop,
		users:      users,
		log:        log,
	}
}

// Refresh rotates a JWT refresh token (RFC 9700).
//
// Validation order:
//  1. DPoP proof (the RT is DPoP-bound; a stolen RT alone cannot refresh).
//  2. RT cryptographic validation — signature, alg, kid, signing-key type,
//     iss/aud/exp/iat/jti/sid, and cnf.jkt == presented DPoP thumbprint.
//     This step is Redis-free, so it stays available during a Redis outage.
//  3. Atomic Redis rotation — the Lua script consumes the current jti and
//     activates a new jti in the same lineage, detecting reuse along the way.
//
// Degraded mode: if Redis is unreachable, no rotation state change is
// performed and a 503 dependency error is returned (never an invalid-token
// error). Access-token validation is unaffected.
func (h *RefreshHandlerImpl) Refresh(c *echo.Context) error {
	ctx := c.Request().Context()

	proof, nonce, err := h.dpop.Validate(ctx, c.Request())
	if err != nil {
		// A plain error (not *dpop.Error) means the DPoP nonce store (Redis)
		// was unreachable — that is a dependency failure, not a bad proof.
		var de *dpop.Error
		if !errors.As(err, &de) {
			return h.dependencyError(c, "refresh: dpop validation unavailable", err)
		}
		return respondDPoPError(c, nonce, err)
	}
	// Issue the fresh nonce BEFORE writing the response: headers set after
	// c.JSON has committed the response are silently dropped, so the client
	// would keep re-using the just-consumed nonce ("invalid or expired DPoP
	// nonce" on the next call).
	setDPoPNonce(c, nonce)

	jkt, err := proof.Thumbprint()
	if err != nil {
		return respondDPoPError(c, nonce, &dpop.Error{Code: dpop.ErrorInvalidProof, Message: "failed to derive DPoP key thumbprint"})
	}

	cookie, err := c.Cookie("refresh")
	if err != nil {
		h.log.Warn("refresh: missing cookie")
		return c.JSON(http.StatusBadRequest, response.Error("missing refresh token"))
	}

	claims, err := h.service.ValidateRefreshToken(ctx, cookie.Value, jkt)
	if err != nil {
		if errors.Is(err, errs.ErrRefreshTokenBindingMismatch) {
			h.log.Warn("refresh: dpop validation failure (rt binding)")
			return respondDPoPError(c, nonce, &dpop.Error{Code: dpop.ErrorInvalidBinding, Status: http.StatusUnauthorized, Message: "DPoP key does not match the bound refresh token"})
		}
		h.log.Warn("refresh: rt validation failure", "error", err)
		return c.JSON(errs.ParseError(err))
	}

	lockKey := "refresh:lock:" + service.HashRefreshToken(cookie.Value)
	lockToken := uuid.Must(uuid.NewV7()).String()
	acquired, err := h.locker.AcquireLock(ctx, lockKey, lockToken, time.Second*5)
	if err != nil {
		return h.dependencyError(c, "refresh: redis unavailable (lock)", err)
	}
	if !acquired {
		h.log.Warn("refresh: lock busy", "user_id", claims.Subject)
		return c.JSON(http.StatusTooManyRequests, response.Error("refresh in progress"))
	}
	defer func() {
		_ = h.locker.ReleaseLock(ctx, lockKey, lockToken)
	}()

	rotated, err := h.service.RotateRefreshToken(ctx, claims)
	if err != nil {
		switch {
		case errors.Is(err, errs.ErrRefreshTokenReused):
			h.log.Warn("refresh: rt reuse detected", "user_id", claims.Subject, "lineage_id", claims.SID)
			if rerr := h.service.RevokeLineage(ctx, claims.SID); rerr != nil {
				h.log.Warn("refresh: lineage revocation failed", "lineage_id", claims.SID, "error", rerr)
			} else {
				h.log.Warn("refresh: lineage revoked", "lineage_id", claims.SID)
			}
		case errors.Is(err, errs.ErrRefreshTokenRevoked):
			h.log.Warn("refresh: lineage revoked", "lineage_id", claims.SID)
		case errors.Is(err, errs.ErrDependencyUnavailable):
			h.log.Warn("refresh: redis unavailable", "user_id", claims.Subject)
		case errors.Is(err, errs.ErrDependencyTimeout):
			h.log.Warn("refresh: redis timeout", "user_id", claims.Subject)
		default:
			h.log.Warn("refresh: rotation failure", "user_id", claims.Subject, "error", err)
		}
		return c.JSON(errs.ParseError(err))
	}

	// Sign the fresh access token with the user's CURRENT role (not a hardcoded
	// role) so role changes take effect immediately and admins are not demoted
	// to USER on every token refresh. A disabled (soft-deleted) user is
	// rejected here because GetUser only returns active accounts.
	user, err := h.users.GetUser(ctx, rotated.UserID)
	if err != nil {
		h.log.Warn("refresh: user unavailable", "user_id", rotated.UserID, "error", err)
		return c.JSON(http.StatusUnauthorized, response.Error("account unavailable"))
	}

	token, err := h.jwtService.NewAccessToken(ctx, rotated.UserID, user.Role, jkt)
	if err != nil {
		return c.JSON(errs.ParseError(err))
	}

	c.SetCookie(&http.Cookie{
		Name:     "refresh",
		Value:    rotated.NewToken,
		Path:     "/api/auth",
		Secure:   false,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Expires:  time.Now().Add(time.Hour * 24 * 7),
	})
	h.log.Info("refresh: success", "user_id", rotated.UserID)
	return c.JSON(http.StatusOK, response.OK(structs.LoginUserResp{
		Token: *token,
	}))
}

// dependencyError maps a Redis dependency failure to a 503 (degraded mode) so
// a Redis outage is never misreported as an invalid refresh token.
func (h *RefreshHandlerImpl) dependencyError(c *echo.Context, msg string, err error) error {
	if errors.Is(err, context.DeadlineExceeded) {
		h.log.Warn("refresh: redis timeout", "error", err)
		return c.JSON(errs.ParseError(errs.ErrDependencyTimeout))
	}
	h.log.Warn("refresh: redis unavailable", "error", err)
	return c.JSON(errs.ParseError(errs.ErrDependencyUnavailable))
}
