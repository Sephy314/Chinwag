package handler_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Sephy314/chinwag/auth/domain"
	"github.com/Sephy314/chinwag/auth/handler"
	"github.com/Sephy314/chinwag/auth/mocked"
	"github.com/Sephy314/chinwag/auth/structs"
	"github.com/Sephy314/chinwag/shared/errs"
	"github.com/Sephy314/chinwag/shared/response"
	"github.com/golang-jwt/jwt/v5"
	"github.com/labstack/echo/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type MockJwtService struct {
	mock.Mock
}

func (m *MockJwtService) NewAccessToken(ctx context.Context, userId string, role domain.Role) (*string, error) {
	args := m.Called(ctx, userId, role)

	var token *string
	if args.Get(0) != nil {
		token = args.Get(0).(*string)
	}

	return token, args.Error(1)
}

func (m *MockJwtService) KeyFunc(token *jwt.Token) (any, error) {
	args := m.Called(token)
	return args.Get(0), args.Error(1)
}

func TestRefreshHandler_Refresh_Success(t *testing.T) {
	e := echo.New()

	userId := "test-user-id"
	refreshTokenValue := "valid-refresh-token"
	newAccessToken := "new-access-token"

	mockRefreshTokenService := &mocked.RefreshTokenService{}
	mockRefreshTokenService.
		On("GetUserIdByRefreshToken", mock.Anything, refreshTokenValue).
		Return(&userId, nil)
	mockRefreshTokenService.
		On("InsertRefreshToken", mock.Anything, mock.MatchedBy(func(rt structs.RefreshToken) bool {
			return rt.Subject == userId && rt.RefreshToken != ""
		})).
		Return(nil)

	mockJwtService := &MockJwtService{}
	mockJwtService.
		On("NewAccessToken", mock.Anything, userId, domain.USER).
		Return(&newAccessToken, nil)

	hdl := handler.NewRefreshHandler(mockRefreshTokenService, mockJwtService)

	req := httptest.NewRequest(http.MethodPost, "/auth/refresh", nil)
	req.AddCookie(&http.Cookie{
		Name:  "refresh",
		Value: refreshTokenValue,
	})

	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := hdl.Refresh(c)

	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)

	var resp response.Response[structs.LoginUserResp]
	err = json.Unmarshal(rec.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Equal(t, true, resp.Success)
	assert.Equal(t, newAccessToken, resp.Data.Token)

	cookies := rec.Result().Cookies()
	refreshCookie := false
	for _, cookie := range cookies {
		if cookie.Name == "refresh" {
			refreshCookie = true
			assert.Equal(t, http.StatusOK, rec.Code)
			assert.True(t, cookie.Secure)
			assert.True(t, cookie.HttpOnly)
			assert.Equal(t, "/auth", cookie.Path)
			assert.Equal(t, http.SameSiteLaxMode, cookie.SameSite)
			break
		}
	}
	assert.True(t, refreshCookie, "refresh cookie should be set")

	mockRefreshTokenService.AssertExpectations(t)
	mockJwtService.AssertExpectations(t)
}

func TestRefreshHandler_Refresh_NoCookie(t *testing.T) {
	e := echo.New()

	mockRefreshTokenService := &mocked.RefreshTokenService{}
	mockJwtService := &MockJwtService{}

	hdl := handler.NewRefreshHandler(mockRefreshTokenService, mockJwtService)

	req := httptest.NewRequest(http.MethodPost, "/auth/refresh", nil)
	rec := httptest.NewRecorder()

	c := e.NewContext(req, rec)

	err := hdl.Refresh(c)

	assert.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, rec.Code)

	mockRefreshTokenService.AssertNotCalled(t, "GetUserIdByRefreshToken", mock.Anything, mock.Anything)
	mockJwtService.AssertNotCalled(t, "NewAccessToken", mock.Anything, mock.Anything, mock.Anything)
}

func TestRefreshHandler_Refresh_InvalidToken(t *testing.T) {
	e := echo.New()

	refreshTokenValue := "invalid-refresh-token"

	mockRefreshTokenService := &mocked.RefreshTokenService{}
	mockRefreshTokenService.
		On("GetUserIdByRefreshToken", mock.Anything, refreshTokenValue).
		Return(nil, errs.ErrNotFound)

	mockJwtService := &MockJwtService{}

	hdl := handler.NewRefreshHandler(mockRefreshTokenService, mockJwtService)

	req := httptest.NewRequest(http.MethodPost, "/auth/refresh", nil)
	req.AddCookie(&http.Cookie{
		Name:  "refresh",
		Value: refreshTokenValue,
	})

	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := hdl.Refresh(c)

	assert.NoError(t, err)
	assert.Equal(t, http.StatusNotFound, rec.Code)

	mockRefreshTokenService.AssertExpectations(t)
}

func TestRefreshHandler_Refresh_InsertRefreshTokenError(t *testing.T) {
	e := echo.New()

	userId := "test-user-id"
	refreshTokenValue := "valid-refresh-token"
	mockRefreshTokenService := &mocked.RefreshTokenService{}
	mockRefreshTokenService.
		On("GetUserIdByRefreshToken", mock.Anything, refreshTokenValue).
		Return(&userId, nil)
	mockRefreshTokenService.
		On("InsertRefreshToken", mock.Anything, mock.Anything).
		Return(errors.New("database error"))

	mockJwtService := &MockJwtService{}
	mockJwtService.
		On("NewAccessToken", mock.Anything, userId, domain.USER).
		Return(new("new-access-token"), nil)

	hdl := handler.NewRefreshHandler(mockRefreshTokenService, mockJwtService)

	req := httptest.NewRequest(http.MethodPost, "/auth/refresh", nil)
	req.AddCookie(&http.Cookie{
		Name:  "refresh",
		Value: refreshTokenValue,
	})

	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := hdl.Refresh(c)

	assert.NoError(t, err)
	assert.Equal(t, http.StatusInternalServerError, rec.Code)

	mockRefreshTokenService.AssertExpectations(t)
	mockJwtService.AssertExpectations(t)
}

func TestRefreshHandler_Refresh_NewAccessTokenError(t *testing.T) {
	e := echo.New()

	userId := "test-user-id"
	refreshTokenValue := "valid-refresh-token"

	mockRefreshTokenService := &mocked.RefreshTokenService{}
	mockRefreshTokenService.
		On("GetUserIdByRefreshToken", mock.Anything, refreshTokenValue).
		Return(&userId, nil)

	mockJwtService := &MockJwtService{}
	mockJwtService.
		On("NewAccessToken", mock.Anything, userId, domain.USER).
		Return(nil, errors.New("jwt error"))

	hdl := handler.NewRefreshHandler(mockRefreshTokenService, mockJwtService)

	req := httptest.NewRequest(http.MethodPost, "/auth/refresh", nil)
	req.AddCookie(&http.Cookie{
		Name:  "refresh",
		Value: refreshTokenValue,
	})

	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := hdl.Refresh(c)

	assert.NoError(t, err)
	assert.Equal(t, http.StatusInternalServerError, rec.Code)

	mockRefreshTokenService.AssertExpectations(t)
	mockJwtService.AssertExpectations(t)
}

func TestRefreshHandler_Refresh_RefreshCookieExpiration(t *testing.T) {
	e := echo.New()

	userId := "test-user-id"
	refreshTokenValue := "valid-refresh-token"
	mockRefreshTokenService := &mocked.RefreshTokenService{}
	mockRefreshTokenService.
		On("GetUserIdByRefreshToken", mock.Anything, refreshTokenValue).
		Return(&userId, nil)
	mockRefreshTokenService.
		On("InsertRefreshToken", mock.Anything, mock.Anything).
		Return(nil)

	mockJwtService := &MockJwtService{}
	mockJwtService.
		On("NewAccessToken", mock.Anything, userId, domain.USER).
		Return(new("new-access-token"), nil)

	hdl := handler.NewRefreshHandler(mockRefreshTokenService, mockJwtService)

	req := httptest.NewRequest(http.MethodPost, "/auth/refresh", nil)
	req.AddCookie(&http.Cookie{
		Name:  "refresh",
		Value: refreshTokenValue,
	})

	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	beforeTime := time.Now()
	err := hdl.Refresh(c)

	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)

	cookies := rec.Result().Cookies()
	var refreshCookie *http.Cookie
	for _, cookie := range cookies {
		if cookie.Name == "refresh" {
			refreshCookie = cookie
			break
		}
	}

	require.NotNil(t, refreshCookie)
	expectedDuration := time.Hour * 24 * 7
	actualDuration := refreshCookie.Expires.Sub(beforeTime)

	assert.True(t, actualDuration >= expectedDuration-time.Minute && actualDuration <= expectedDuration+time.Minute,
		"Refresh cookie expiration should be approximately 7 days from now")
}
