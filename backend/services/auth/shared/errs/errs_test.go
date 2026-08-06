package errs

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net"
	"net/http"
	"testing"

	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/labstack/echo/v5"
	"github.com/stretchr/testify/assert"
	"golang.org/x/crypto/bcrypt"
)

func TestParseError_DBConnectionErrors(t *testing.T) {
	// Database is down / connection broken → must map to a clean 500, never
	// leak the raw error to clients.
	cases := []struct {
		name string
		err  error
	}{
		{
			name: "sql.ErrConnDone",
			err:  sql.ErrConnDone,
		},
		{
			name: "context deadline exceeded",
			err:  context.DeadlineExceeded,
		},
		{
			name: "connection refused (net.OpError)",
			err: &net.OpError{
				Op:  "dial",
				Net: "tcp",
				Err: errors.New("connection refused"),
			},
		},
		{
			name: "wrapped connect failure",
			err:  fmt.Errorf("failed to connect to db: %w", context.DeadlineExceeded),
		},
		{
			name: "wrapped connection error",
			err:  errors.Join(context.DeadlineExceeded, sql.ErrConnDone),
		},
		{
			name: "generic unknown error",
			err:  errors.New("boom"),
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			status, resp := ParseError(tc.err)

			assert.Equal(t, http.StatusInternalServerError, status)
			assert.False(t, resp.Success)
			assert.Equal(t, "Internal Server Error", resp.Message)
		})
	}
}

func TestParseError_DBRecordErrors(t *testing.T) {
	t.Run("no rows → 404", func(t *testing.T) {
		status, resp := ParseError(sql.ErrNoRows)
		assert.Equal(t, http.StatusNotFound, status)
		assert.Equal(t, ErrNotFound.Message, resp.Message)
	})

	t.Run("unique violation → 409", func(t *testing.T) {
		status, resp := ParseError(&pgconn.PgError{Code: pgerrcode.UniqueViolation})
		assert.Equal(t, http.StatusConflict, status)
		assert.Equal(t, ErrConflict.Message, resp.Message)
	})

	t.Run("other pg error → 500", func(t *testing.T) {
		status, _ := ParseError(&pgconn.PgError{Code: pgerrcode.TooManyConnections})
		assert.Equal(t, http.StatusInternalServerError, status)
	})
}

func TestParseError_HTTPErrors(t *testing.T) {
	t.Run("echo unauthorized", func(t *testing.T) {
		status, resp := ParseError(echo.ErrUnauthorized)
		assert.Equal(t, http.StatusUnauthorized, status)
		assert.Equal(t, "Unauthorized", resp.Message)
	})

	t.Run("custom echo http error", func(t *testing.T) {
		status, resp := ParseError(echo.NewHTTPError(http.StatusTeapot, "short and stout"))
		assert.Equal(t, http.StatusTeapot, status)
		assert.Equal(t, "short and stout", resp.Message)
	})
}

func TestParseError_AuthErrors(t *testing.T) {
	t.Run("bcrypt mismatch → 400 invalid creds", func(t *testing.T) {
		status, resp := ParseError(bcrypt.ErrMismatchedHashAndPassword)
		assert.Equal(t, http.StatusBadRequest, status)
		assert.Equal(t, "Invalid Creds", resp.Message)
	})

	t.Run("app error passthrough", func(t *testing.T) {
		status, resp := ParseError(ErrRefreshTokenReused)
		assert.Equal(t, http.StatusUnauthorized, status)
		assert.Equal(t, "Refresh token reuse detected", resp.Message)
	})
}
