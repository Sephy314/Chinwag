package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/golang-jwt/jwt/v5"
	"github.com/labstack/echo/v5"
	"github.com/stretchr/testify/require"
)

func withClaims(c *echo.Context, role, subject string) {
	claims := &Claims{Role: role, RegisteredClaims: jwt.RegisteredClaims{Subject: subject}}
	c.Set(ClaimsContextKey, claims)
}

func TestRequireRole_NoClaims_Unauthorized(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/admin/test", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	handler := RequireRole(RoleAdmin)(func(c *echo.Context) error {
		t.Fatal("handler should not be called")
		return nil
	})
	_ = handler(c)

	require.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestRequireRole_WrongRole_Forbidden(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/admin/test", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	withClaims(c, RoleUser, "u1")

	handler := RequireRole(RoleAdmin)(func(c *echo.Context) error {
		t.Fatal("handler should not be called")
		return nil
	})
	_ = handler(c)

	require.Equal(t, http.StatusForbidden, rec.Code)
}

func TestRequireRole_ManagerForbiddenForAdminOnly(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/admin/test", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	withClaims(c, RoleManager, "u1")

	handler := RequireRole(RoleAdmin)(func(c *echo.Context) error {
		t.Fatal("handler should not be called")
		return nil
	})
	_ = handler(c)

	require.Equal(t, http.StatusForbidden, rec.Code)
}

func TestRequireRole_AdminAllowed(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/admin/test", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	withClaims(c, RoleAdmin, "admin1")

	called := false
	handler := RequireRole(RoleAdmin)(func(c *echo.Context) error {
		called = true
		return c.String(http.StatusOK, "ok")
	})
	err := handler(c)
	require.NoError(t, err)
	require.True(t, called)
	require.Equal(t, http.StatusOK, rec.Code)
}
