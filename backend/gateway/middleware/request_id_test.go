package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"regexp"
	"testing"

	appMiddleware "github.com/Sephy314/chinwag/backend/gateway/middleware"
	"github.com/labstack/echo/v5"
	"github.com/stretchr/testify/require"
)

func TestRequestID_GeneratesUUIDWhenMissing(t *testing.T) {
	e := echo.New()

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	var capturedRID string
	handler := appMiddleware.RequestID()(func(c *echo.Context) error {
		rid, _ := echo.ContextGet[string](c, appMiddleware.ContextKeyRequestID)
		capturedRID = rid
		return c.String(http.StatusOK, "ok")
	})

	err := handler(c)
	require.NoError(t, err)
	require.NotEmpty(t, capturedRID)

	uuidRe := regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)
	require.Regexp(t, uuidRe, capturedRID)

	require.Equal(t, capturedRID, req.Header.Get(appMiddleware.HeaderXRequestID))
}

func TestRequestID_PreservesExistingHeader(t *testing.T) {
	e := echo.New()

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set(appMiddleware.HeaderXRequestID, "my-custom-id-123")
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	var capturedRID string
	handler := appMiddleware.RequestID()(func(c *echo.Context) error {
		rid, _ := echo.ContextGet[string](c, appMiddleware.ContextKeyRequestID)
		capturedRID = rid
		return c.String(http.StatusOK, "ok")
	})

	err := handler(c)
	require.NoError(t, err)
	require.Equal(t, "my-custom-id-123", capturedRID)
	require.Equal(t, "my-custom-id-123", req.Header.Get(appMiddleware.HeaderXRequestID))
}

func TestRequestID_SetsHeaderOnRequest(t *testing.T) {
	e := echo.New()

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	handler := appMiddleware.RequestID()(func(c *echo.Context) error {
		return c.String(http.StatusOK, "ok")
	})

	err := handler(c)
	require.NoError(t, err)

	headerVal := req.Header.Get(appMiddleware.HeaderXRequestID)
	require.NotEmpty(t, headerVal)
}
