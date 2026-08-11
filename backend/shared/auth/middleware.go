package auth

import (
	"errors"
	"net/http"
	"strings"

	"github.com/Sephy314/chinwag/backend/shared/auth/dpop"
	"github.com/golang-jwt/jwt/v5"
	"github.com/labstack/echo/v5"
)

const ClaimsContextKey = "auth_claims"

const dpopNonceHeader = "DPoP-Nonce"

// Global user roles, mirroring services/auth/domain (User.Role). Kept here so
// shared middleware can enforce role checks without importing the auth module.
const (
	RoleUser    = "USER"
	RoleManager = "MANAGER"
	RoleAdmin   = "ADMIN"
)

// RequireRole returns a middleware that rejects requests whose authenticated
// user does not hold one of the given roles. It must be composed AFTER
// NewMiddleware (which stores the verified Claims in the echo context).
//   - no verified claims  -> 401 Unauthorized
//   - valid claims, wrong role -> 403 Forbidden
func RequireRole(roles ...string) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) error {
			claims, err := ClaimsFromContext(c)
			if err != nil {
				return c.JSON(http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
			}
			for _, r := range roles {
				if claims.Role == r {
					return next(c)
				}
			}
			return c.JSON(http.StatusForbidden, map[string]string{"error": "forbidden"})
		}
	}
}

// NewMiddleware validates the DPoP bearer access token and requires a
// sender-constrained DPoP proof bound to the token's cnf.jkt claim (RFC 9449).
// The shared validator applies the same proof/nonce/replay checks used by the
// token-issuing endpoints. If validator is nil, DPoP validation is skipped
// with a warning — production deployments must always pass a validator.
func NewMiddleware(client *JWKSClient, log Logger, validator *dpop.Validator) echo.MiddlewareFunc {
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

			if validator != nil {
				if derr := verifyDPoP(c, claims, validator); derr != nil {
					return respondDPoPError(c, derr)
				}
			} else {
				log.Info("DPoP validation disabled because no validator was provided")
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

func verifyDPoP(c *echo.Context, claims *Claims, v *dpop.Validator) *dpop.Error {
	if claims.CNF == nil || claims.CNF.Jkt == "" {
		return &dpop.Error{
			Code:    dpop.ErrorInvalidBinding,
			Status:  http.StatusUnauthorized,
			Message: "access token is not bound to a DPoP key",
		}
	}

	proof, nonce, err := v.Validate(c.Request().Context(), c.Request())
	if nonce != "" {
		c.Response().Header().Set(dpopNonceHeader, nonce)
	}
	if err != nil {
		var de *dpop.Error
		if errors.As(err, &de) {
			return de
		}
		return &dpop.Error{
			Code:    dpop.ErrorInvalidProof,
			Status:  http.StatusInternalServerError,
			Message: "DPoP validation unavailable: " + err.Error(),
		}
	}

	jkt, err := proof.Thumbprint()
	if err != nil {
		return &dpop.Error{
			Code:    dpop.ErrorInvalidProof,
			Status:  http.StatusBadRequest,
			Message: "failed to derive DPoP key thumbprint",
		}
	}
	if jkt != claims.CNF.Jkt {
		return &dpop.Error{
			Code:    dpop.ErrorInvalidBinding,
			Status:  http.StatusUnauthorized,
			Message: "DPoP key does not match the bound access token",
		}
	}

	return nil
}

func respondDPoPError(c *echo.Context, derr *dpop.Error) error {
	return c.JSON(derr.Status, map[string]string{
		"error": derr.Message,
		"code":  derr.Code,
	})
}
