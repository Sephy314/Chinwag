package middleware_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	appMiddleware "github.com/Sephy314/chinwag/middleware"
	"github.com/Sephy314/chinwag/shared/response"
	"github.com/labstack/echo/v5"
	"github.com/stretchr/testify/require"
)

func TestRequestIDInjector_WithHeader(t *testing.T) {
	e := echo.New()

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.Response().Header().Set(echo.HeaderXRequestID, "abc-123")

	handler := appMiddleware.RequestIDInjector()(func(c *echo.Context) error {
		rid, err := echo.ContextGet[string](c, response.RequestIDKey)
		require.NoError(t, err)
		require.Equal(t, "abc-123", rid)
		return c.JSON(http.StatusOK, response.OK[any](nil))
	})

	err := handler(c)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, rec.Code)

	var resp response.Response[any]
	err = json.Unmarshal(rec.Body.Bytes(), &resp)
	require.NoError(t, err)
	require.Equal(t, true, resp.Success)
}

func TestRequestIDInjector_WithoutHeader(t *testing.T) {
	e := echo.New()

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	handler := appMiddleware.RequestIDInjector()(func(c *echo.Context) error {
		_, err := echo.ContextGet[string](c, response.RequestIDKey)
		require.Error(t, err)
		return c.JSON(http.StatusOK, response.OK[any](nil))
	})

	err := handler(c)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, rec.Code)
}

func TestResponseIDInjector_InjectsRequestID(t *testing.T) {
	e := echo.New()

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	c.Set(response.RequestIDKey, "req-456")

	handler := appMiddleware.ResponseIDInjector()(func(c *echo.Context) error {
		return c.JSON(http.StatusOK, response.OK(map[string]string{"foo": "bar"}))
	})

	err := handler(c)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, rec.Code)

	var resp response.Response[map[string]string]
	err = json.Unmarshal(rec.Body.Bytes(), &resp)
	require.NoError(t, err)
	require.True(t, resp.Success)
	require.Equal(t, "req-456", resp.RequestID)
	require.Equal(t, "bar", resp.Data["foo"])
}

func TestResponseIDInjector_NoRequestID_PassesThrough(t *testing.T) {
	e := echo.New()

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	handler := appMiddleware.ResponseIDInjector()(func(c *echo.Context) error {
		return c.JSON(http.StatusOK, response.OK(map[string]string{"foo": "bar"}))
	})

	err := handler(c)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, rec.Code)

	var resp response.Response[map[string]string]
	err = json.Unmarshal(rec.Body.Bytes(), &resp)
	require.NoError(t, err)
	require.True(t, resp.Success)
	require.Equal(t, "", resp.RequestID)
	require.Equal(t, "bar", resp.Data["foo"])
}

func TestResponseIDInjector_HandlerError(t *testing.T) {
	e := echo.New()

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	c.Set(response.RequestIDKey, "req-789")

	handler := appMiddleware.ResponseIDInjector()(func(c *echo.Context) error {
		return echo.ErrNotFound
	})

	err := handler(c)
	require.NoError(t, err)
	require.Equal(t, http.StatusNotFound, rec.Code)

	var resp response.Response[any]
	err = json.Unmarshal(rec.Body.Bytes(), &resp)
	require.NoError(t, err)
	require.False(t, resp.Success)
	require.Equal(t, "req-789", resp.RequestID)
}
