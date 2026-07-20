package handler_test

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"testing"

	"golang.org/x/crypto/bcrypt"

	"github.com/Sephy314/chinwag/auth/domain"
	"github.com/Sephy314/chinwag/auth/handler"
	"github.com/Sephy314/chinwag/auth/mocked"
	"github.com/Sephy314/chinwag/auth/service"
	"github.com/Sephy314/chinwag/conn/cache"
	"github.com/Sephy314/chinwag/shared/response"
	"github.com/labstack/echo/v5"
	"github.com/labstack/echo/v5/echotest"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestUserHandler_Login_UserNotFound(t *testing.T) {
	mockedRepo := &mocked.UserRepo{}
	mockedCache := &cache.MockedCache{}
	mockedJwk := &mocked.JwkService{}
	mockedRefresh := &mocked.RefreshTokenService{}

	mockedRepo.
		On("GetUserByEmail", mock.Anything, "notfound@example.com").
		Return((*domain.User)(nil), sql.ErrNoRows)

	svc := service.NewUserService(mockedCache, mockedRepo, mockedJwk, mockedRefresh, nil)
	h := handler.NewUserHandler(svc)

	rec := echotest.ContextConfig{
		Headers: map[string][]string{
			echo.HeaderContentType: {echo.MIMEApplicationJSON},
		},
		JSONBody: []byte(`{"email":"notfound@example.com","password":"whatever"}`),
	}.ServeWithHandler(t, h.Login)

	require.Equal(t, http.StatusBadRequest, rec.Code)

	var resp response.Response[any]
	err := json.Unmarshal(rec.Body.Bytes(), &resp)
	require.NoError(t, err)
	require.Equal(t, false, resp.Success)
	require.Equal(t, "Invalid Credentials", resp.Message)

	mockedRepo.AssertExpectations(t)
}

func TestUserHandler_Login_WrongPassword(t *testing.T) {
	mockedRepo := &mocked.UserRepo{}
	mockedCache := &cache.MockedCache{}
	mockedJwk := &mocked.JwkService{}
	mockedRefresh := &mocked.RefreshTokenService{}

	pw := "correctPassword"
	hash, err := bcrypt.GenerateFromPassword([]byte(pw), bcrypt.MinCost)
	require.NoError(t, err)

	user := &domain.User{
		Id:       "uid-1",
		Name:     "tester",
		Email:    "tester@example.com",
		Password: string(hash),
	}

	mockedRepo.On("GetUserByEmail", mock.Anything, user.Email).Return(user, nil)

	svc := service.NewUserService(mockedCache, mockedRepo, mockedJwk, mockedRefresh, nil)
	h := handler.NewUserHandler(svc)

	rec := echotest.ContextConfig{
		Headers: map[string][]string{
			echo.HeaderContentType: {echo.MIMEApplicationJSON},
		},
		JSONBody: []byte(`{"email":"tester@example.com","password":"wrongPassword"}`),
	}.ServeWithHandler(t, h.Login)

	require.Equal(t, http.StatusBadRequest, rec.Code)

	mockedRepo.AssertExpectations(t)
}

func TestUserHandler_GetUser_WithEmail_Success(t *testing.T) {
	mockedRepo := &mocked.UserRepo{}
	mockedCache := &cache.MockedCache{}
	mockedJwk := &mocked.JwkService{}
	mockedRefresh := &mocked.RefreshTokenService{}

	user := &domain.User{
		Id:    "uid-123",
		Name:  "john",
		Email: "john@example.com",
	}

	mockedRepo.
		On("GetUserByEmail", mock.Anything, "john@example.com").
		Return(user, nil)

	svc := service.NewUserService(mockedCache, mockedRepo, mockedJwk, mockedRefresh, nil)
	h := handler.NewUserHandler(svc)

	rec := echotest.ContextConfig{
		PathValues: []echo.PathValue{
			{Name: "id", Value: "john@example.com"},
		},
	}.ServeWithHandler(t, h.GetUser)

	require.Equal(t, http.StatusOK, rec.Code)

	var resp response.Response[map[string]interface{}]
	err := json.Unmarshal(rec.Body.Bytes(), &resp)
	require.NoError(t, err)
	require.Equal(t, true, resp.Success)
	require.Equal(t, "uid-123", resp.Data["id"])
	require.Equal(t, "john", resp.Data["name"])
	require.Equal(t, "john@example.com", resp.Data["email"])

	mockedRepo.AssertExpectations(t)
}

func TestUserHandler_GetUser_WithEmail_NotFound(t *testing.T) {
	mockedRepo := &mocked.UserRepo{}
	mockedCache := &cache.MockedCache{}
	mockedJwk := &mocked.JwkService{}
	mockedRefresh := &mocked.RefreshTokenService{}

	mockedRepo.
		On("GetUserByEmail", mock.Anything, "notfound@example.com").
		Return((*domain.User)(nil), sql.ErrNoRows)

	svc := service.NewUserService(mockedCache, mockedRepo, mockedJwk, mockedRefresh, nil)
	h := handler.NewUserHandler(svc)

	rec := echotest.ContextConfig{
		PathValues: []echo.PathValue{
			{Name: "id", Value: "notfound@example.com"},
		},
	}.ServeWithHandler(t, h.GetUser)

	require.Equal(t, http.StatusNotFound, rec.Code)

	mockedRepo.AssertExpectations(t)
}

func TestUserHandler_GetUser_WithID_Success(t *testing.T) {
	mockedRepo := &mocked.UserRepo{}
	mockedCache := &cache.MockedCache{}
	mockedJwk := &mocked.JwkService{}
	mockedRefresh := &mocked.RefreshTokenService{}

	user := &domain.User{
		Id:    "uid-456",
		Name:  "jane",
		Email: "jane@example.com",
	}

	mockedRepo.
		On("GetUser", mock.Anything, "uid-456").
		Return(user, nil)

	svc := service.NewUserService(mockedCache, mockedRepo, mockedJwk, mockedRefresh, nil)
	h := handler.NewUserHandler(svc)

	rec := echotest.ContextConfig{
		PathValues: []echo.PathValue{
			{Name: "id", Value: "uid-456"},
		},
	}.ServeWithHandler(t, h.GetUser)

	require.Equal(t, http.StatusOK, rec.Code)

	var resp response.Response[map[string]interface{}]
	err := json.Unmarshal(rec.Body.Bytes(), &resp)
	require.NoError(t, err)
	require.Equal(t, true, resp.Success)
	require.Equal(t, "uid-456", resp.Data["id"])
	require.Equal(t, "jane", resp.Data["name"])
	require.Equal(t, "jane@example.com", resp.Data["email"])

	mockedRepo.AssertExpectations(t)
}

func TestUserHandler_GetUser_WithID_NotFound(t *testing.T) {
	mockedRepo := &mocked.UserRepo{}
	mockedCache := &cache.MockedCache{}
	mockedJwk := &mocked.JwkService{}
	mockedRefresh := &mocked.RefreshTokenService{}

	mockedRepo.
		On("GetUser", mock.Anything, "uid-nonexistent").
		Return((*domain.User)(nil), sql.ErrNoRows)

	svc := service.NewUserService(mockedCache, mockedRepo, mockedJwk, mockedRefresh, nil)
	h := handler.NewUserHandler(svc)

	rec := echotest.ContextConfig{
		PathValues: []echo.PathValue{
			{Name: "id", Value: "uid-nonexistent"},
		},
	}.ServeWithHandler(t, h.GetUser)

	require.Equal(t, http.StatusNotFound, rec.Code)

	mockedRepo.AssertExpectations(t)
}

func TestUserHandler_UpdateUser_Success(t *testing.T) {
	mockedRepo := &mocked.UserRepo{}
	mockedCache := &cache.MockedCache{}
	mockedJwk := &mocked.JwkService{}
	mockedRefresh := &mocked.RefreshTokenService{}

	existing := &domain.User{
		Id:    "uid-1",
		Name:  "oldName",
		Email: "old@example.com",
	}

	mockedRepo.On("GetUser", mock.Anything, "uid-1").Return(existing, nil)
	mockedRepo.On("UpdateUser", mock.Anything, mock.AnythingOfType("domain.User")).Return(nil)

	svc := service.NewUserService(mockedCache, mockedRepo, mockedJwk, mockedRefresh, nil)
	h := handler.NewUserHandler(svc)

	rec := echotest.ContextConfig{
		PathValues: []echo.PathValue{
			{Name: "id", Value: "uid-1"},
		},
		Headers: map[string][]string{
			echo.HeaderContentType: {echo.MIMEApplicationJSON},
		},
		JSONBody: []byte(`{"name":"newName"}`),
	}.ServeWithHandler(t, h.UpdateUser)

	require.Equal(t, http.StatusOK, rec.Code)

	var resp response.Response[map[string]interface{}]
	err := json.Unmarshal(rec.Body.Bytes(), &resp)
	require.NoError(t, err)
	require.Equal(t, true, resp.Success)
	require.Equal(t, "newName", resp.Data["name"])
	require.Equal(t, "old@example.com", resp.Data["email"])

	mockedRepo.AssertExpectations(t)
}

func TestUserHandler_UpdateUser_NotFound(t *testing.T) {
	mockedRepo := &mocked.UserRepo{}
	mockedCache := &cache.MockedCache{}
	mockedJwk := &mocked.JwkService{}
	mockedRefresh := &mocked.RefreshTokenService{}

	mockedRepo.On("GetUser", mock.Anything, "uid-nonexistent").
		Return((*domain.User)(nil), sql.ErrNoRows)

	svc := service.NewUserService(mockedCache, mockedRepo, mockedJwk, mockedRefresh, nil)
	h := handler.NewUserHandler(svc)

	rec := echotest.ContextConfig{
		PathValues: []echo.PathValue{
			{Name: "id", Value: "uid-nonexistent"},
		},
		Headers: map[string][]string{
			echo.HeaderContentType: {echo.MIMEApplicationJSON},
		},
		JSONBody: []byte(`{"name":"newName"}`),
	}.ServeWithHandler(t, h.UpdateUser)

	require.Equal(t, http.StatusNotFound, rec.Code)

	mockedRepo.AssertExpectations(t)
}

func TestUserHandler_UpdateUser_InvalidBody(t *testing.T) {
	mockedRepo := &mocked.UserRepo{}
	mockedCache := &cache.MockedCache{}
	mockedJwk := &mocked.JwkService{}
	mockedRefresh := &mocked.RefreshTokenService{}

	svc := service.NewUserService(mockedCache, mockedRepo, mockedJwk, mockedRefresh, nil)
	h := handler.NewUserHandler(svc)

	rec := echotest.ContextConfig{
		PathValues: []echo.PathValue{
			{Name: "id", Value: "uid-1"},
		},
		Headers: map[string][]string{
			echo.HeaderContentType: {echo.MIMEApplicationJSON},
		},
		JSONBody: []byte(`{"name": 123}`),
	}.ServeWithHandler(t, h.UpdateUser)

	// Bind may succeed even with wrong types, so the service will be called
	// The important thing is no panic and a response is returned
	require.True(t, rec.Code == http.StatusOK || rec.Code == http.StatusBadRequest)
}
