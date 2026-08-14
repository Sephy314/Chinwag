package errs

import (
	"database/sql"
	"errors"
	"net/http"

	"github.com/Sephy314/chinwag/backend/services/auth/shared/response"
	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/labstack/echo/v5"
	"golang.org/x/crypto/bcrypt"
)

type AppError struct {
	Status  int    `json:"status"`
	Message string `json:"message"`
}

func (e *AppError) Error() string {
	return e.Message
}

var (
	ErrConflict = &AppError{
		Status:  http.StatusConflict,
		Message: "User already exists",
	}

	ErrNotFound = &AppError{
		Status:  http.StatusNotFound,
		Message: "User not found",
	}

	ErrCacheNotFound = &AppError{
		Status:  http.StatusNotFound,
		Message: "Cached data not found",
	}

	ErrInvalidCreds = &AppError{
		Status:  http.StatusBadRequest,
		Message: "Invalid Credentials",
	}

	InvalidAlgErr = &AppError{
		Status:  http.StatusBadRequest,
		Message: "Algorithm invalid",
	}

	InvalidTokenErr = &AppError{
		Status:  http.StatusBadRequest,
		Message: "Invalid token",
	}

	ErrNoKey = &AppError{
		Status:  http.StatusBadRequest,
		Message: "No key",
	}

	ErrRefreshTokenReused = &AppError{
		Status:  http.StatusUnauthorized,
		Message: "Refresh token reuse detected",
	}

	ErrRefreshTokenRevoked = &AppError{
		Status:  http.StatusUnauthorized,
		Message: "Refresh token revoked",
	}

	// ErrInvalidRefreshToken is returned when a refresh-token JWT fails
	// cryptographic validation (parse, alg, signature, or claims).
	ErrInvalidRefreshToken = &AppError{
		Status:  http.StatusUnauthorized,
		Message: "Invalid refresh token",
	}

	// ErrInvalidGrant is returned when a cryptographically valid refresh token
	// has no valid Redis grant state (e.g. its state expired or is missing).
	ErrInvalidGrant = &AppError{
		Status:  http.StatusUnauthorized,
		Message: "Invalid grant",
	}

	// ErrRefreshTokenBindingMismatch is returned when the presented DPoP key
	// does not match the refresh token's cnf.jkt claim.
	ErrRefreshTokenBindingMismatch = &AppError{
		Status:  http.StatusUnauthorized,
		Message: "Refresh token DPoP binding mismatch",
	}

	// ErrDependencyUnavailable signals a degraded mode: the Redis dependency
	// could not be reached, so rotation/reuse detection could not run. It must
	// never be conflated with "invalid refresh token".
	ErrDependencyUnavailable = &AppError{
		Status:  http.StatusServiceUnavailable,
		Message: "Refresh service temporarily unavailable",
	}

	// ErrDependencyTimeout signals a degraded mode where Redis timed out.
	ErrDependencyTimeout = &AppError{
		Status:  http.StatusServiceUnavailable,
		Message: "Refresh service temporarily unavailable",
	}

	// ErrRotationFailed signals that a new refresh token could not be produced
	// (signing key unavailable or signing failure). No Redis state changed.
	ErrRotationFailed = &AppError{
		Status:  http.StatusInternalServerError,
		Message: "Refresh rotation failed",
	}

	ErrInvalidRole = &AppError{
		Status:  http.StatusBadRequest,
		Message: "Invalid role",
	}

	ErrSelfDemotion = &AppError{
		Status:  http.StatusBadRequest,
		Message: "Cannot demote yourself",
	}

	ErrLastAdmin = &AppError{
		Status:  http.StatusConflict,
		Message: "Cannot remove the last administrator",
	}
)

func ParseError(err error) (int, *response.Response[any]) {
	if he, ok := errors.AsType[*echo.HTTPError](err); ok {
		msg := he.Message
		if msg == "" {
			msg = http.StatusText(he.Code)
		}
		return he.Code, response.Error(msg)
	}

	if code := echo.StatusCode(err); code != 0 {
		return code, response.Error(http.StatusText(code))
	}

	listErrParsers := []func(error) error{
		parseAuthError,
		parseDBError,
	}

	for _, parser := range listErrParsers {
		parsed := parser(err)

		if a, ok := errors.AsType[*AppError](parsed); ok {
			return a.Status, response.Error(a.Message)
		}
	}

	return http.StatusInternalServerError, response.Error("Internal Server Error")
}

func parseAuthError(err error) error {
	if errors.Is(err, bcrypt.ErrMismatchedHashAndPassword) {
		return &AppError{
			Status:  http.StatusBadRequest,
			Message: "Invalid Creds",
		}
	}

	return err
}

func parseDBError(err error) error {
	switch {
	case errors.Is(err, sql.ErrNoRows):
		return ErrNotFound
	}

	if pgErr, ok := errors.AsType[*pgconn.PgError](err); ok {
		switch pgErr.Code {
		case pgerrcode.UniqueViolation:
			return ErrConflict
		}
	}

	return err
}
