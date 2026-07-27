package errs

import (
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"testing"

	"github.com/labstack/echo/v5"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"
)

func TestParseError_EchoHTTPError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		wantCode int
		wantMsg  string
	}{
		{
			name:     "echo.ErrUnauthorized returns 401",
			err:      echo.ErrUnauthorized,
			wantCode: http.StatusUnauthorized,
			wantMsg:  http.StatusText(http.StatusUnauthorized),
		},
		{
			name:     "echo.ErrForbidden returns 403",
			err:      echo.ErrForbidden,
			wantCode: http.StatusForbidden,
			wantMsg:  http.StatusText(http.StatusForbidden),
		},
		{
			name:     "echo.ErrNotFound returns 404",
			err:      echo.ErrNotFound,
			wantCode: http.StatusNotFound,
			wantMsg:  http.StatusText(http.StatusNotFound),
		},
		{
			name:     "echo.ErrBadRequest returns 400",
			err:      echo.ErrBadRequest,
			wantCode: http.StatusBadRequest,
			wantMsg:  http.StatusText(http.StatusBadRequest),
		},
		{
			name:     "echo.ErrTooManyRequests returns 429",
			err:      echo.ErrTooManyRequests,
			wantCode: http.StatusTooManyRequests,
			wantMsg:  http.StatusText(http.StatusTooManyRequests),
		},
		{
			name:     "NewHTTPError with custom message",
			err:      echo.NewHTTPError(http.StatusTooManyRequests, "rate limited"),
			wantCode: http.StatusTooManyRequests,
			wantMsg:  "rate limited",
		},
		{
			name:     "wrapped echo ErrUnauthorized",
			err:      fmt.Errorf("wrapper: %w", echo.ErrUnauthorized),
			wantCode: http.StatusUnauthorized,
			wantMsg:  http.StatusText(http.StatusUnauthorized),
		},
		{
			name:     "wrapped echo ErrForbidden",
			err:      fmt.Errorf("auth failed: %w", echo.ErrForbidden),
			wantCode: http.StatusForbidden,
			wantMsg:  http.StatusText(http.StatusForbidden),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			code, resp := ParseError(tt.err)
			require.Equal(t, tt.wantCode, code)
			require.Equal(t, false, resp.Success)
			require.Equal(t, tt.wantMsg, resp.Message)
		})
	}
}

func TestParseError_AppError(t *testing.T) {
	err := &AppError{
		Status:  http.StatusConflict,
		Message: "User already exists",
	}

	code, resp := ParseError(err)
	require.Equal(t, http.StatusConflict, code)
	require.Equal(t, false, resp.Success)
	require.Equal(t, "User already exists", resp.Message)
}

func TestParseError_BcryptError(t *testing.T) {
	code, resp := ParseError(bcrypt.ErrMismatchedHashAndPassword)
	require.Equal(t, http.StatusBadRequest, code)
	require.Equal(t, false, resp.Success)
	require.Equal(t, "Invalid Creds", resp.Message)
}

func TestParseError_SQLNoRows(t *testing.T) {
	code, resp := ParseError(sql.ErrNoRows)
	require.Equal(t, http.StatusNotFound, code)
	require.Equal(t, false, resp.Success)
	require.Equal(t, "User not found", resp.Message)
}

func TestParseError_UnknownError(t *testing.T) {
	err := errors.New("something unexpected")
	code, resp := ParseError(err)
	require.Equal(t, http.StatusInternalServerError, code)
	require.Equal(t, false, resp.Success)
	require.Equal(t, "Internal Server Error", resp.Message)
}
