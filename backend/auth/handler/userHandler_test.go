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
