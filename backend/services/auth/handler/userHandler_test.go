package handler

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"testing"

	"github.com/Sephy314/chinwag/backend/services/auth/domain"
	"github.com/Sephy314/chinwag/backend/services/auth/shared/errs"
	"github.com/Sephy314/chinwag/backend/services/auth/shared/response"
	"github.com/Sephy314/chinwag/backend/services/auth/structs"
	sharedauth "github.com/Sephy314/chinwag/backend/shared/auth"
	"github.com/Sephy314/chinwag/backend/shared/auth/dpop"
	"github.com/golang-jwt/jwt/v5"
	"github.com/labstack/echo/v5"
	"github.com/labstack/echo/v5/echotest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestUserHandler_Health(t *testing.T) {
	h := newUserHandlerWith(new(MockUserRepo), new(MockJwksService), new(MockRefreshTokenService), nil)

	c, rec := echotest.ContextConfig{}.ToContextRecorder(t)
	err := h.Health(c)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)

	var resp response.Response[any]
	assert.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.True(t, resp.Success)
}

func TestUserHandler_CreateUser_Success(t *testing.T) {
	userRepo := new(MockUserRepo)
	h := newUserHandlerWith(userRepo, new(MockJwksService), new(MockRefreshTokenService), nil)

	userRepo.On("CreateUser", mock.Anything, mock.MatchedBy(func(u domain.User) bool {
		return u.Name == "Alice" && u.Email == "alice@example.com"
	})).Return(nil).Once()

	c, rec := echotest.ContextConfig{
		Headers: map[string][]string{
			echo.HeaderContentType: {echo.MIMEApplicationJSON},
		},
		JSONBody: []byte(`{"name":"Alice","email":"alice@example.com","password":"secret"}`),
	}.ToContextRecorder(t)

	err := h.CreateUser(c)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)

	var resp response.Response[structs.UserResponse]
	assert.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Equal(t, "Alice", resp.Data.Name)
	userRepo.AssertExpectations(t)
}

func TestUserHandler_CreateUser_InvalidBody(t *testing.T) {
	userRepo := new(MockUserRepo)
	h := newUserHandlerWith(userRepo, new(MockJwksService), new(MockRefreshTokenService), nil)

	c, rec := echotest.ContextConfig{
		Headers: map[string][]string{
			echo.HeaderContentType: {echo.MIMEApplicationJSON},
		},
		JSONBody: []byte(`{bad`),
	}.ToContextRecorder(t)

	err := h.CreateUser(c)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	userRepo.AssertNotCalled(t, "CreateUser", mock.Anything, mock.Anything)
}

func TestUserHandler_CreateUser_Conflict(t *testing.T) {
	userRepo := new(MockUserRepo)
	h := newUserHandlerWith(userRepo, new(MockJwksService), new(MockRefreshTokenService), nil)

	userRepo.On("CreateUser", mock.Anything, mock.Anything).Return(errs.ErrConflict).Once()

	c, rec := echotest.ContextConfig{
		Headers: map[string][]string{
			echo.HeaderContentType: {echo.MIMEApplicationJSON},
		},
		JSONBody: []byte(`{"name":"Alice","email":"alice@example.com","password":"secret"}`),
	}.ToContextRecorder(t)

	err := h.CreateUser(c)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusConflict, rec.Code)
	userRepo.AssertExpectations(t)
}

func TestUserHandler_GetUserByID_Success(t *testing.T) {
	userRepo := new(MockUserRepo)
	h := newUserHandlerWith(userRepo, new(MockJwksService), new(MockRefreshTokenService), nil)

	expected := &domain.User{Id: "u1", Name: "Alice", Email: "a@example.com"}
	userRepo.On("GetUser", mock.Anything, "u1").Return(expected, nil).Once()

	c, rec := echotest.ContextConfig{
		PathValues: []echo.PathValue{{Name: "id", Value: "u1"}},
	}.ToContextRecorder(t)

	err := h.GetUserByID(c)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)

	var resp response.Response[structs.UserResponse]
	assert.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Equal(t, "Alice", resp.Data.Name)
	userRepo.AssertExpectations(t)
}

func TestUserHandler_GetUserByID_NotFound(t *testing.T) {
	userRepo := new(MockUserRepo)
	h := newUserHandlerWith(userRepo, new(MockJwksService), new(MockRefreshTokenService), nil)

	userRepo.On("GetUser", mock.Anything, "u1").Return(nil, errs.ErrNotFound).Once()

	c, rec := echotest.ContextConfig{
		PathValues: []echo.PathValue{{Name: "id", Value: "u1"}},
	}.ToContextRecorder(t)

	err := h.GetUserByID(c)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusNotFound, rec.Code)
	userRepo.AssertExpectations(t)
}

func TestUserHandler_GetUserByEmail_Success(t *testing.T) {
	userRepo := new(MockUserRepo)
	h := newUserHandlerWith(userRepo, new(MockJwksService), new(MockRefreshTokenService), nil)

	expected := &domain.User{Id: "u1", Email: "a@example.com"}
	userRepo.On("GetUserByEmail", mock.Anything, "a@example.com").Return(expected, nil).Once()

	c, rec := echotest.ContextConfig{
		PathValues: []echo.PathValue{{Name: "email", Value: "a@example.com"}},
	}.ToContextRecorder(t)

	err := h.GetUserByEmail(c)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)
	userRepo.AssertExpectations(t)
}

func TestUserHandler_GetUserByEmail_NotFound(t *testing.T) {
	userRepo := new(MockUserRepo)
	h := newUserHandlerWith(userRepo, new(MockJwksService), new(MockRefreshTokenService), nil)

	userRepo.On("GetUserByEmail", mock.Anything, "nope@example.com").Return(nil, errs.ErrNotFound).Once()

	c, rec := echotest.ContextConfig{
		PathValues: []echo.PathValue{{Name: "email", Value: "nope@example.com"}},
	}.ToContextRecorder(t)

	err := h.GetUserByEmail(c)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusNotFound, rec.Code)
	userRepo.AssertExpectations(t)
}

func TestUserHandler_DeleteUser_Success(t *testing.T) {
	userRepo := new(MockUserRepo)
	h := newUserHandlerWith(userRepo, new(MockJwksService), new(MockRefreshTokenService), nil)

	userRepo.On("DeleteUser", mock.Anything, "u1").Return(nil).Once()

	c, rec := echotest.ContextConfig{
		PathValues: []echo.PathValue{{Name: "id", Value: "u1"}},
	}.ToContextRecorder(t)

	err := h.DeleteUser(c)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)
	userRepo.AssertExpectations(t)
}

func TestUserHandler_DeleteUser_Error(t *testing.T) {
	userRepo := new(MockUserRepo)
	h := newUserHandlerWith(userRepo, new(MockJwksService), new(MockRefreshTokenService), nil)

	userRepo.On("DeleteUser", mock.Anything, "u1").Return(errors.New("db down")).Once()

	c, rec := echotest.ContextConfig{
		PathValues: []echo.PathValue{{Name: "id", Value: "u1"}},
	}.ToContextRecorder(t)

	err := h.DeleteUser(c)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
	userRepo.AssertExpectations(t)
}

func TestUserHandler_UpdateUser_Success(t *testing.T) {
	userRepo := new(MockUserRepo)
	h := newUserHandlerWith(userRepo, new(MockJwksService), new(MockRefreshTokenService), nil)

	userRepo.On("GetUser", mock.Anything, "u1").Return(&domain.User{Id: "u1", Name: "Old", Email: "a@example.com"}, nil).Once()
	userRepo.On("UpdateUser", mock.Anything, mock.MatchedBy(func(u domain.User) bool {
		return u.Name == "NewName"
	})).Return(nil).Once()

	c, rec := echotest.ContextConfig{
		PathValues: []echo.PathValue{{Name: "id", Value: "u1"}},
		Headers: map[string][]string{
			echo.HeaderContentType: {echo.MIMEApplicationJSON},
		},
		JSONBody: []byte(`{"name":"NewName"}`),
	}.ToContextRecorder(t)

	err := h.UpdateUser(c)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)

	var resp response.Response[structs.UserResponse]
	assert.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Equal(t, "NewName", resp.Data.Name)
	userRepo.AssertExpectations(t)
}

func TestUserHandler_UpdateUser_InvalidBody(t *testing.T) {
	userRepo := new(MockUserRepo)
	h := newUserHandlerWith(userRepo, new(MockJwksService), new(MockRefreshTokenService), nil)

	c, rec := echotest.ContextConfig{
		PathValues: []echo.PathValue{{Name: "id", Value: "u1"}},
		Headers: map[string][]string{
			echo.HeaderContentType: {echo.MIMEApplicationJSON},
		},
		JSONBody: []byte(`{bad`),
	}.ToContextRecorder(t)

	err := h.UpdateUser(c)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	userRepo.AssertNotCalled(t, "UpdateUser", mock.Anything, mock.Anything)
}

func TestUserHandler_WhoAmI_Success(t *testing.T) {
	userRepo := new(MockUserRepo)
	h := newUserHandlerWith(userRepo, new(MockJwksService), new(MockRefreshTokenService), nil)

	expected := &domain.User{Id: "u1", Name: "Alice", Email: "a@example.com"}
	userRepo.On("GetUser", mock.Anything, "u1").Return(expected, nil).Once()

	c, rec := echotest.ContextConfig{}.ToContextRecorder(t)
	c.Set(sharedauth.ClaimsContextKey, &sharedauth.Claims{RegisteredClaims: jwt.RegisteredClaims{Subject: "u1"}})

	err := h.WhoAmI(c)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)

	var resp response.Response[map[string]any]
	assert.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	userRepo.AssertExpectations(t)
}

func TestUserHandler_WhoAmI_NoClaims(t *testing.T) {
	userRepo := new(MockUserRepo)
	h := newUserHandlerWith(userRepo, new(MockJwksService), new(MockRefreshTokenService), nil)

	c, rec := echotest.ContextConfig{}.ToContextRecorder(t)

	err := h.WhoAmI(c)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
	userRepo.AssertNotCalled(t, "GetUser", mock.Anything, mock.Anything)
}

func TestUserHandler_Login_Success(t *testing.T) {
	userRepo := new(MockUserRepo)
	jwk := new(MockJwksService)
	refresh := new(MockRefreshTokenService)
	dpopSvc := new(MockDPoPService)
	h := newUserHandlerWith(userRepo, jwk, refresh, dpopSvc)

	proof := makeProof(t)

	dpopSvc.On("Validate", mock.Anything, mock.Anything).Return(proof, "nonce-1", nil).Once()
	refresh.On("IssueRefreshToken", mock.Anything, "u1", mock.Anything, "").Return("new-refresh-token", nil).Once()
	userRepo.On("GetUserByEmail", mock.Anything, "alice@example.com").Return(
		&domain.User{Id: "u1", Email: "alice@example.com", Password: mustHash(t, "secret")}, nil,
	).Once()
	jwk.On("GetActiveAccessKey", mock.Anything).Return(makeSigningKey(t), nil).Once()

	c, rec := echotest.ContextConfig{
		Headers: map[string][]string{
			echo.HeaderContentType: {echo.MIMEApplicationJSON},
		},
		JSONBody: []byte(`{"email":"alice@example.com","password":"secret"}`),
	}.ToContextRecorder(t)

	err := h.Login(c)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)

	var resp response.Response[map[string]string]
	assert.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.NotEmpty(t, resp.Data["token"])
	dpopSvc.AssertExpectations(t)
	userRepo.AssertExpectations(t)
	jwk.AssertExpectations(t)
	refresh.AssertExpectations(t)
}

func TestUserHandler_Login_InvalidBody(t *testing.T) {
	userRepo := new(MockUserRepo)
	dpopSvc := new(MockDPoPService)
	h := newUserHandlerWith(userRepo, new(MockJwksService), new(MockRefreshTokenService), dpopSvc)

	c, rec := echotest.ContextConfig{
		Headers: map[string][]string{
			echo.HeaderContentType: {echo.MIMEApplicationJSON},
		},
		JSONBody: []byte(`{bad`),
	}.ToContextRecorder(t)

	err := h.Login(c)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	dpopSvc.AssertNotCalled(t, "Validate", mock.Anything, mock.Anything)
}

func TestUserHandler_Login_DPoPError(t *testing.T) {
	userRepo := new(MockUserRepo)
	dpopSvc := new(MockDPoPService)
	h := newUserHandlerWith(userRepo, new(MockJwksService), new(MockRefreshTokenService), dpopSvc)

	dpopSvc.On("Validate", mock.Anything, mock.Anything).Return(nil, "nonce-1", &dpop.Error{
		Code:    dpop.ErrorInvalidProof,
		Status:  http.StatusBadRequest,
		Message: "invalid proof",
	}).Once()

	c, rec := echotest.ContextConfig{
		Headers: map[string][]string{
			echo.HeaderContentType: {echo.MIMEApplicationJSON},
		},
		JSONBody: []byte(`{"email":"alice@example.com","password":"secret"}`),
	}.ToContextRecorder(t)

	err := h.Login(c)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Equal(t, "nonce-1", rec.Header().Get(dpop.NonceHeader))
	dpopSvc.AssertExpectations(t)
}

func TestUserHandler_Logout(t *testing.T) {
	h := newUserHandlerWith(new(MockUserRepo), new(MockJwksService), new(MockRefreshTokenService), nil)

	c, rec := echotest.ContextConfig{}.ToContextRecorder(t)

	err := h.Logout(c)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)

	cookies := rec.Result().Cookies()
	var refreshCookie *http.Cookie
	for _, ck := range cookies {
		if ck.Name == "refresh" {
			refreshCookie = ck
			break
		}
	}
	assert.NotNil(t, refreshCookie)
	assert.Equal(t, -1, refreshCookie.MaxAge)
}

func TestUserHandler_GetUserByID_DBConnError(t *testing.T) {
	userRepo := new(MockUserRepo)
	h := newUserHandlerWith(userRepo, new(MockJwksService), new(MockRefreshTokenService), nil)

	// DB is down: the service propagates sql.ErrConnDone and the handler must
	// turn it into a clean 500 response without leaking the raw error.
	userRepo.On("GetUser", mock.Anything, "u1").Return(nil, sql.ErrConnDone).Once()

	c, rec := echotest.ContextConfig{
		PathValues: []echo.PathValue{{Name: "id", Value: "u1"}},
	}.ToContextRecorder(t)

	err := h.GetUserByID(c)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusInternalServerError, rec.Code)

	var resp response.Response[any]
	assert.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.False(t, resp.Success)
	assert.Equal(t, "Internal Server Error", resp.Message)
	assert.NotContains(t, rec.Body.String(), "sql:")
	userRepo.AssertExpectations(t)
}

func TestUserHandler_CreateUser_DBConnError(t *testing.T) {
	userRepo := new(MockUserRepo)
	h := newUserHandlerWith(userRepo, new(MockJwksService), new(MockRefreshTokenService), nil)

	userRepo.On("CreateUser", mock.Anything, mock.Anything).Return(sql.ErrConnDone).Once()

	c, rec := echotest.ContextConfig{
		Headers: map[string][]string{
			echo.HeaderContentType: {echo.MIMEApplicationJSON},
		},
		JSONBody: []byte(`{"name":"Alice","email":"alice@example.com","password":"secret"}`),
	}.ToContextRecorder(t)

	err := h.CreateUser(c)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
	userRepo.AssertExpectations(t)
}
