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
}

func TestParseError_AppErrors(t *testing.T) {
	t.Run("room popped passthrough → 410", func(t *testing.T) {
		status, resp := ParseError(ErrRoomPopped)
		assert.Equal(t, http.StatusGone, status)
		assert.Equal(t, ErrRoomPopped.Message, resp.Message)
	})

	t.Run("bcrypt mismatch → 400", func(t *testing.T) {
		status, resp := ParseError(bcrypt.ErrMismatchedHashAndPassword)
		assert.Equal(t, http.StatusBadRequest, status)
		assert.Equal(t, "Invalid Creds", resp.Message)
	})
}
