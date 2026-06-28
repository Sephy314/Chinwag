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
	"github.com/labstack/echo/v5"
	"github.com/labstack/echo/v5/echotest"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// Case: login with email that does not exist -> 404 User not found
func TestUserHandler_Login_UserNotFound(t *testing.T) {
	mockedRepo := &mocked.UserRepo{}
	mockedCache := &cache.MockedCache{}
	mockedJwk := &mocked.JwkService{}
	mockedRefresh := &mocked.RefreshTokenService{}

	mockedRepo.
		On("GetUser", mock.Anything, "notfound@example.com").
		Return((*domain.User)(nil), sql.ErrNoRows)

	svc := service.NewUserService(mockedCache, mockedRepo, mockedJwk, mockedRefresh)
	h := handler.NewUserHandler(svc)

	rec := echotest.ContextConfig{
		Headers: map[string][]string{
			echo.HeaderContentType: {echo.MIMEApplicationJSON},
		},
		JSONBody: []byte(`{"email":"notfound@example.com","password":"whatever"}`),
	}.ServeWithHandler(t, h.Login)

	require.Equal(t, http.StatusBadRequest, rec.Code)

	var resp string
	err := json.Unmarshal(rec.Body.Bytes(), &resp)
	require.NoError(t, err)

	mockedRepo.AssertExpectations(t)
}

// Case: login with wrong password -> service returns bcrypt mismatch -> 500
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

	mockedRepo.On("GetUser", mock.Anything, user.Email).Return(user, nil)

	svc := service.NewUserService(mockedCache, mockedRepo, mockedJwk, mockedRefresh)
	h := handler.NewUserHandler(svc)

	rec := echotest.ContextConfig{
		Headers: map[string][]string{
			echo.HeaderContentType: {echo.MIMEApplicationJSON},
		},
		JSONBody: []byte(`{"email":"tester@example.com","password":"wrongPassword"}`),
	}.ServeWithHandler(t, h.Login)

	// bcrypt mismatch should return 400 (Invalid Creds)
	require.Equal(t, http.StatusBadRequest, rec.Code)

	mockedRepo.AssertExpectations(t)
}

// Case: GetUser with email format -> success
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

	svc := service.NewUserService(mockedCache, mockedRepo, mockedJwk, mockedRefresh)
	h := handler.NewUserHandler(svc)

	rec := echotest.ContextConfig{
		PathValues: []echo.PathValue{
			{Name: "id", Value: "john@example.com"},
		},
	}.ServeWithHandler(t, h.GetUser)

	require.Equal(t, http.StatusOK, rec.Code)

	var resp map[string]interface{}
	err := json.Unmarshal(rec.Body.Bytes(), &resp)
	require.NoError(t, err)
	require.Equal(t, "uid-123", resp["id"])
	require.Equal(t, "john", resp["name"])
	require.Equal(t, "john@example.com", resp["email"])

	mockedRepo.AssertExpectations(t)
}

// Case: GetUser with email format -> user not found
func TestUserHandler_GetUser_WithEmail_NotFound(t *testing.T) {
	mockedRepo := &mocked.UserRepo{}
	mockedCache := &cache.MockedCache{}
	mockedJwk := &mocked.JwkService{}
	mockedRefresh := &mocked.RefreshTokenService{}

	mockedRepo.
		On("GetUserByEmail", mock.Anything, "notfound@example.com").
		Return((*domain.User)(nil), sql.ErrNoRows)

	svc := service.NewUserService(mockedCache, mockedRepo, mockedJwk, mockedRefresh)
	h := handler.NewUserHandler(svc)

	rec := echotest.ContextConfig{
		PathValues: []echo.PathValue{
			{Name: "id", Value: "notfound@example.com"},
		},
	}.ServeWithHandler(t, h.GetUser)

	require.Equal(t, http.StatusNotFound, rec.Code)

	mockedRepo.AssertExpectations(t)
}

// Case: GetUser with id format -> success
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

	svc := service.NewUserService(mockedCache, mockedRepo, mockedJwk, mockedRefresh)
	h := handler.NewUserHandler(svc)

	rec := echotest.ContextConfig{
		PathValues: []echo.PathValue{
			{Name: "id", Value: "uid-456"},
		},
	}.ServeWithHandler(t, h.GetUser)

	require.Equal(t, http.StatusOK, rec.Code)

	var resp map[string]interface{}
	err := json.Unmarshal(rec.Body.Bytes(), &resp)
	require.NoError(t, err)
	require.Equal(t, "uid-456", resp["id"])
	require.Equal(t, "jane", resp["name"])
	require.Equal(t, "jane@example.com", resp["email"])

	mockedRepo.AssertExpectations(t)
}

// Case: GetUser with id format -> user not found
func TestUserHandler_GetUser_WithID_NotFound(t *testing.T) {
	mockedRepo := &mocked.UserRepo{}
	mockedCache := &cache.MockedCache{}
	mockedJwk := &mocked.JwkService{}
	mockedRefresh := &mocked.RefreshTokenService{}

	mockedRepo.
		On("GetUser", mock.Anything, "uid-nonexistent").
		Return((*domain.User)(nil), sql.ErrNoRows)

	svc := service.NewUserService(mockedCache, mockedRepo, mockedJwk, mockedRefresh)
	h := handler.NewUserHandler(svc)

	rec := echotest.ContextConfig{
		PathValues: []echo.PathValue{
			{Name: "id", Value: "uid-nonexistent"},
		},
	}.ServeWithHandler(t, h.GetUser)

	require.Equal(t, http.StatusNotFound, rec.Code)

	mockedRepo.AssertExpectations(t)
}
