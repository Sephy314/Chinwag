package auth

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/Sephy314/chinwag/backend/shared/auth/dpop"
	"github.com/golang-jwt/jwt/v5"
	"github.com/labstack/echo/v5"
)

const ClaimsContextKey = "auth_claims"

// SetNXStore provides an atomic key-once insertion, used to detect DPoP proof
// jti replay. It matches the go-redis SetNX behavior via an adapter.
type SetNXStore interface {
	SetNX(ctx context.Context, key string, value any, ttl time.Duration) (bool, error)
}

// DPoPError carries an RFC 9449 error code and its HTTP status.
type DPoPError struct {
	Code    string
	Status  int
	Message string
}

func (e *DPoPError) Error() string { return e.Message }

const (
	dpopProofMaxAge = 60 * time.Second
	dpopFutureSkew  = 60 * time.Second
	dpopJtiTTL      = 2 * time.Minute
)

// NewMiddleware validates the bearer/DPoP access token and, when a replay
// store is provided, requires a sender-constrained DPoP proof bound to the
// token's cnf.jkt claim (RFC 9449). If store is nil, DPoP validation is
// skipped with a warning — production deployments must always pass a store.
func NewMiddleware(client *JWKSClient, log Logger, store SetNXStore) echo.MiddlewareFunc {
	client.SetLogger(log)

	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) error {
			tokenString, err := parseAuthorization(c)
			if err != nil {
				return c.JSON(http.StatusUnauthorized, map[string]string{"error": err.Error()})
			}

			claims := &Claims{}
			token, err := jwt.ParseWithClaims(tokenString, claims, client.KeyFunc())
			if err != nil || !token.Valid {
				return c.JSON(http.StatusUnauthorized, map[string]string{"error": "invalid or expired token"})
			}

			if store != nil {
				if derr := verifyDPoP(c, claims, store); derr != nil {
					return respondDPoPError(c, derr)
				}
			} else {
				log.Info("DPoP validation disabled because no replay store was provided")
			}

			c.Set(ClaimsContextKey, claims)
			c.Set("user", token)

			return next(c)
		}
	}
}

func parseAuthorization(c *echo.Context) (string, error) {
	authHeader := c.Request().Header.Get("Authorization")
	if authHeader == "" {
		return "", echo.ErrUnauthorized
	}

	parts := strings.SplitN(authHeader, " ", 2)
	if len(parts) != 2 {
		return "", echo.ErrUnauthorized
	}

	scheme := strings.ToLower(parts[0])
	if scheme != "bearer" && scheme != "dpop" {
		return "", echo.ErrUnauthorized
	}

	return parts[1], nil
}

func verifyDPoP(c *echo.Context, claims *Claims, store SetNXStore) *DPoPError {
	if claims.CNF == nil || claims.CNF.Jkt == "" {
		return &DPoPError{
			Code:    dpop.ErrorInvalidBinding,
			Status:  http.StatusUnauthorized,
			Message: "access token is not bound to a DPoP key",
		}
	}

	raw := c.Request().Header.Get(dpop.HeaderName)
	if raw == "" {
		return &DPoPError{
			Code:    dpop.ErrorInvalidProof,
			Status:  http.StatusBadRequest,
			Message: "missing DPoP proof",
		}
	}

	proof, err := dpop.ParseProof(raw)
	if err != nil {
		return &DPoPError{
			Code:    dpop.ErrorInvalidProof,
			Status:  http.StatusBadRequest,
			Message: "invalid DPoP proof: " + err.Error(),
		}
	}

	htu := dpop.RequestHTU(c.Request())
	if err := proof.Validate(c.Request().Method, htu, time.Now(), dpopProofMaxAge, dpopFutureSkew); err != nil {
		return &DPoPError{
			Code:    dpop.ErrorInvalidProof,
			Status:  http.StatusBadRequest,
			Message: err.Error(),
		}
	}

	if err := proof.VerifySignature(); err != nil {
		return &DPoPError{
			Code:    dpop.ErrorInvalidProof,
			Status:  http.StatusBadRequest,
			Message: "invalid DPoP proof signature",
		}
	}

	acquired, err := store.SetNX(c.Request().Context(), "dpop:jti:"+proof.Claims.Jti, "1", dpopJtiTTL)
	if err != nil {
		return &DPoPError{
			Code:    dpop.ErrorInvalidProof,
			Status:  http.StatusInternalServerError,
			Message: "replay cache unavailable",
		}
	}
	if !acquired {
		return &DPoPError{
			Code:    dpop.ErrorInvalidProof,
			Status:  http.StatusBadRequest,
			Message: "DPoP proof replay detected",
		}
	}

	jkt, err := proof.Thumbprint()
	if err != nil {
		return &DPoPError{
			Code:    dpop.ErrorInvalidProof,
			Status:  http.StatusBadRequest,
			Message: "failed to derive DPoP key thumbprint",
		}
	}
	if jkt != claims.CNF.Jkt {
		return &DPoPError{
			Code:    dpop.ErrorInvalidBinding,
			Status:  http.StatusUnauthorized,
			Message: "DPoP key does not match the bound access token",
		}
	}

	return nil
}

func respondDPoPError(c *echo.Context, derr *DPoPError) error {
	return c.JSON(derr.Status, map[string]string{
		"error": derr.Message,
		"code":  derr.Code,
	})
}
