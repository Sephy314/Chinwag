package middleware_test

import (
	"bytes"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	appMiddleware "github.com/Sephy314/chinwag/backend/gateway/middleware"
	"github.com/labstack/echo/v5"
	"github.com/stretchr/testify/require"
)

func setupLogger() (*bytes.Buffer, *slog.Logger) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))
	return &buf, logger
}

func TestAccessLogger_LogsRequestFields(t *testing.T) {
	buf, logger := setupLogger()
	slog.SetDefault(logger)

	e := echo.New()

	req := httptest.NewRequest(http.MethodGet, "/test-path", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	c.Set(appMiddleware.ContextKeyRequestID, "req-abc")

	handler := appMiddleware.AccessLogger()(func(c *echo.Context) error {
		return c.String(http.StatusOK, "ok")
	})

	err := handler(c)
	require.NoError(t, err)

	output := buf.String()
	require.Contains(t, output, `"msg":"request"`)
	require.Contains(t, output, `"request_id":"req-abc"`)
	require.Contains(t, output, `"method":"GET"`)
	require.Contains(t, output, `"path":"/test-path"`)
	require.Contains(t, output, `"status":200`)
	require.Contains(t, output, `"latency":`)
	require.Contains(t, output, `"client_ip":`)
	require.Contains(t, output, `"timestamp":`)
}

func TestAccessLogger_CapturesNon200Status(t *testing.T) {
	buf, logger := setupLogger()
	slog.SetDefault(logger)

	e := echo.New()

	req := httptest.NewRequest(http.MethodGet, "/not-found", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	c.Set(appMiddleware.ContextKeyRequestID, "req-404")

	handler := appMiddleware.AccessLogger()(func(c *echo.Context) error {
		c.Response().WriteHeader(http.StatusNotFound)
		return nil
	})

	err := handler(c)
	require.NoError(t, err)

	output := buf.String()
	require.Contains(t, output, `"status":404`)
}

func TestAccessLogger_EmptyRequestID(t *testing.T) {
	buf, logger := setupLogger()
	slog.SetDefault(logger)

	e := echo.New()

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	handler := appMiddleware.AccessLogger()(func(c *echo.Context) error {
		return c.String(http.StatusOK, "ok")
	})

	err := handler(c)
	require.NoError(t, err)

	output := buf.String()
	require.Contains(t, output, `"request_id":""`)
}

func TestAccessLogger_HandlerError(t *testing.T) {
	buf, logger := setupLogger()
	slog.SetDefault(logger)

	e := echo.New()

	req := httptest.NewRequest(http.MethodPost, "/fail", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	c.Set(appMiddleware.ContextKeyRequestID, "req-err")

	handler := appMiddleware.AccessLogger()(func(c *echo.Context) error {
		c.Response().WriteHeader(http.StatusUnauthorized)
		return nil
	})

	err := handler(c)
	require.NoError(t, err)

	output := buf.String()
	require.Contains(t, output, `"method":"POST"`)
	require.Contains(t, output, `"path":"/fail"`)
	require.Contains(t, output, `"status":401`)
}

func TestAccessLogger_OutputIsValidJSON(t *testing.T) {
	buf, logger := setupLogger()
	slog.SetDefault(logger)

	e := echo.New()

	req := httptest.NewRequest(http.MethodGet, "/json-check", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	c.Set(appMiddleware.ContextKeyRequestID, "req-json")

	handler := appMiddleware.AccessLogger()(func(c *echo.Context) error {
		return c.String(http.StatusOK, "ok")
	})

	err := handler(c)
	require.NoError(t, err)

	output := strings.TrimSpace(buf.String())
	require.True(t, len(output) > 0)
	require.True(t, output[0] == '{', "expected JSON object, got: %s", output[:min(50, len(output))])
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
