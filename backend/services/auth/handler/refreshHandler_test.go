package handler

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Sephy314/chinwag/backend/services/auth/domain"
	"github.com/Sephy314/chinwag/backend/services/auth/service"
	"github.com/Sephy314/chinwag/backend/services/auth/shared/errs"
	"github.com/Sephy314/chinwag/backend/services/auth/shared/logger"
	"github.com/Sephy314/chinwag/backend/services/auth/shared/response"
	"github.com/Sephy314/chinwag/backend/services/auth/structs"
	"github.com/Sephy314/chinwag/backend/shared/auth/dpop"
	"github.com/labstack/echo/v5"
	"github.com/labstack/echo/v5/echotest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

const testRefreshCookie = "old-refresh-token"

func refreshRequest(t *testing.T, withCookie bool) (*echo.Context, *httptest.ResponseRecorder) {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/auth/refresh", nil)
	if withCookie {
		req.AddCookie(&http.Cookie{Name: "refresh", Value: testRefreshCookie})
	}
	return echotest.ContextConfig{Request: req}.ToContextRecorder(t)
}

func newRefreshHandler(refresh *MockRefreshTokenService, jwtSvc *MockJwtService, locker *MockCache, dpopSvc *MockDPoPService) *RefreshHandlerImpl {
	return NewRefreshHandler(refresh, jwtSvc, locker, dpopSvc, logger.New())
}

func TestRefreshHandler_Refresh_Success(t *testing.T) {
	refresh := new(MockRefreshTokenService)
	jwtSvc := new(MockJwtService)
	locker := new(MockCache)
	dpopSvc := new(MockDPoPService)
	h := newRefreshHandler(refresh, jwtSvc, locker, dpopSvc)

	proof := makeProof(t)
	jkt := proofJkt(t, proof)
	lockKey := "refresh:lock:" + service.HashRefreshToken(testRefreshCookie)
	token := "new-access-token"

	dpopSvc.On("Validate", mock.Anything, mock.Anything).Return(proof, "nonce-1", nil).Once()
	refresh.On("GetRefreshToken", mock.Anything, testRefreshCookie).Return(&structs.RefreshTokenRecord{
		UserID:    "u1",
		LineageID: "lin1",
		Jkt:       jkt,
	}, nil).Once()
	locker.On("AcquireLock", mock.Anything, lockKey, mock.Anything, 5*time.Second).Return(true, nil).Once()
	refresh.On("ConsumeRefreshToken", mock.Anything, testRefreshCookie).Return(&structs.RefreshTokenRecord{
		UserID:    "u1",
		LineageID: "lin1",
	}, nil).Once()
	jwtSvc.On("NewAccessToken", mock.Anything, "u1", domain.USER, jkt).Return(&token, nil).Once()
	refresh.On("InsertRefreshToken", mock.Anything, mock.MatchedBy(func(tk structs.RefreshToken) bool {
		return tk.Subject == "u1" && tk.LineageID == "lin1" && tk.Jkt == jkt && tk.ParentHash == service.HashRefreshToken(testRefreshCookie)
	})).Return(nil).Once()
	locker.On("ReleaseLock", mock.Anything, lockKey, mock.Anything).Return(nil).Once()

	c, rec := refreshRequest(t, true)
	err := h.Refresh(c)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)

	var resp response.Response[structs.LoginUserResp]
	assert.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Equal(t, token, resp.Data.Token)
	refresh.AssertExpectations(t)
	jwtSvc.AssertExpectations(t)
	locker.AssertExpectations(t)
	dpopSvc.AssertExpectations(t)
}

func TestRefreshHandler_Refresh_MissingCookie(t *testing.T) {
	refresh := new(MockRefreshTokenService)
	jwtSvc := new(MockJwtService)
	locker := new(MockCache)
	dpopSvc := new(MockDPoPService)
	h := newRefreshHandler(refresh, jwtSvc, locker, dpopSvc)

	proof := makeProof(t)
	dpopSvc.On("Validate", mock.Anything, mock.Anything).Return(proof, "nonce-1", nil).Once()

	c, rec := refreshRequest(t, false)
	err := h.Refresh(c)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	refresh.AssertNotCalled(t, "GetRefreshToken", mock.Anything, mock.Anything)
	locker.AssertNotCalled(t, "AcquireLock", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
	dpopSvc.AssertExpectations(t)
}

func TestRefreshHandler_Refresh_DPoPError(t *testing.T) {
	refresh := new(MockRefreshTokenService)
	jwtSvc := new(MockJwtService)
	locker := new(MockCache)
	dpopSvc := new(MockDPoPService)
	h := newRefreshHandler(refresh, jwtSvc, locker, dpopSvc)

	dpopSvc.On("Validate", mock.Anything, mock.Anything).Return(nil, "nonce-1", &dpop.Error{
		Code:    dpop.ErrorInvalidProof,
		Status:  http.StatusBadRequest,
		Message: "bad proof",
	}).Once()

	c, rec := refreshRequest(t, true)
	err := h.Refresh(c)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Equal(t, "nonce-1", rec.Header().Get(dpop.NonceHeader))
	refresh.AssertNotCalled(t, "GetRefreshToken", mock.Anything, mock.Anything)
	dpopSvc.AssertExpectations(t)
}

func TestRefreshHandler_Refresh_GetTokenError(t *testing.T) {
	refresh := new(MockRefreshTokenService)
	jwtSvc := new(MockJwtService)
	locker := new(MockCache)
	dpopSvc := new(MockDPoPService)
	h := newRefreshHandler(refresh, jwtSvc, locker, dpopSvc)

	proof := makeProof(t)
	dpopSvc.On("Validate", mock.Anything, mock.Anything).Return(proof, "nonce-1", nil).Once()
	refresh.On("GetRefreshToken", mock.Anything, testRefreshCookie).Return(nil, errs.ErrCacheNotFound).Once()

	c, rec := refreshRequest(t, true)
	err := h.Refresh(c)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusNotFound, rec.Code)
	locker.AssertNotCalled(t, "AcquireLock", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
	refresh.AssertExpectations(t)
	dpopSvc.AssertExpectations(t)
}

func TestRefreshHandler_Refresh_JktMismatch(t *testing.T) {
	refresh := new(MockRefreshTokenService)
	jwtSvc := new(MockJwtService)
	locker := new(MockCache)
	dpopSvc := new(MockDPoPService)
	h := newRefreshHandler(refresh, jwtSvc, locker, dpopSvc)

	proof := makeProof(t)
	dpopSvc.On("Validate", mock.Anything, mock.Anything).Return(proof, "nonce-1", nil).Once()
	refresh.On("GetRefreshToken", mock.Anything, testRefreshCookie).Return(&structs.RefreshTokenRecord{
		UserID:    "u1",
		LineageID: "lin1",
		Jkt:       "some-other-thumbprint",
	}, nil).Once()

	c, rec := refreshRequest(t, true)
	err := h.Refresh(c)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	locker.AssertNotCalled(t, "AcquireLock", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
	refresh.AssertExpectations(t)
	dpopSvc.AssertExpectations(t)
}

func TestRefreshHandler_Refresh_LockBusy(t *testing.T) {
	refresh := new(MockRefreshTokenService)
	jwtSvc := new(MockJwtService)
	locker := new(MockCache)
	dpopSvc := new(MockDPoPService)
	h := newRefreshHandler(refresh, jwtSvc, locker, dpopSvc)

	proof := makeProof(t)
	jkt := proofJkt(t, proof)
	lockKey := "refresh:lock:" + service.HashRefreshToken(testRefreshCookie)

	dpopSvc.On("Validate", mock.Anything, mock.Anything).Return(proof, "nonce-1", nil).Once()
	refresh.On("GetRefreshToken", mock.Anything, testRefreshCookie).Return(&structs.RefreshTokenRecord{
		UserID: "u1", LineageID: "lin1", Jkt: jkt,
	}, nil).Once()
	locker.On("AcquireLock", mock.Anything, lockKey, mock.Anything, 5*time.Second).Return(false, nil).Once()

	c, rec := refreshRequest(t, true)
	err := h.Refresh(c)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusTooManyRequests, rec.Code)
	refresh.AssertNotCalled(t, "ConsumeRefreshToken", mock.Anything, mock.Anything)
	refresh.AssertExpectations(t)
	locker.AssertExpectations(t)
	dpopSvc.AssertExpectations(t)
}

func TestRefreshHandler_Refresh_ReusedTokenRevokesLineage(t *testing.T) {
	refresh := new(MockRefreshTokenService)
	jwtSvc := new(MockJwtService)
	locker := new(MockCache)
	dpopSvc := new(MockDPoPService)
	h := newRefreshHandler(refresh, jwtSvc, locker, dpopSvc)

	proof := makeProof(t)
	jkt := proofJkt(t, proof)
	lockKey := "refresh:lock:" + service.HashRefreshToken(testRefreshCookie)

	dpopSvc.On("Validate", mock.Anything, mock.Anything).Return(proof, "nonce-1", nil).Once()
	refresh.On("GetRefreshToken", mock.Anything, testRefreshCookie).Return(&structs.RefreshTokenRecord{
		UserID: "u1", LineageID: "lin1", Jkt: jkt,
	}, nil).Once()
	locker.On("AcquireLock", mock.Anything, lockKey, mock.Anything, 5*time.Second).Return(true, nil).Once()
	refresh.On("ConsumeRefreshToken", mock.Anything, testRefreshCookie).Return(&structs.RefreshTokenRecord{
		UserID: "u1", LineageID: "lin1", Used: true,
	}, errs.ErrRefreshTokenReused).Once()
	refresh.On("RevokeLineage", mock.Anything, "lin1").Return(nil).Once()
	locker.On("ReleaseLock", mock.Anything, lockKey, mock.Anything).Return(nil).Once()

	c, rec := refreshRequest(t, true)
	err := h.Refresh(c)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
	refresh.AssertExpectations(t)
	locker.AssertExpectations(t)
	dpopSvc.AssertExpectations(t)
}

func TestRefreshHandler_Refresh_NewAccessTokenError(t *testing.T) {
	refresh := new(MockRefreshTokenService)
	jwtSvc := new(MockJwtService)
	locker := new(MockCache)
	dpopSvc := new(MockDPoPService)
	h := newRefreshHandler(refresh, jwtSvc, locker, dpopSvc)

	proof := makeProof(t)
	jkt := proofJkt(t, proof)
	lockKey := "refresh:lock:" + service.HashRefreshToken(testRefreshCookie)

	dpopSvc.On("Validate", mock.Anything, mock.Anything).Return(proof, "nonce-1", nil).Once()
	refresh.On("GetRefreshToken", mock.Anything, testRefreshCookie).Return(&structs.RefreshTokenRecord{
		UserID: "u1", LineageID: "lin1", Jkt: jkt,
	}, nil).Once()
	locker.On("AcquireLock", mock.Anything, lockKey, mock.Anything, 5*time.Second).Return(true, nil).Once()
	refresh.On("ConsumeRefreshToken", mock.Anything, testRefreshCookie).Return(&structs.RefreshTokenRecord{
		UserID: "u1", LineageID: "lin1",
	}, nil).Once()
	jwtSvc.On("NewAccessToken", mock.Anything, "u1", domain.USER, jkt).Return(nil, errs.ErrNoKey).Once()
	locker.On("ReleaseLock", mock.Anything, lockKey, mock.Anything).Return(nil).Once()

	c, rec := refreshRequest(t, true)
	err := h.Refresh(c)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	refresh.AssertExpectations(t)
	jwtSvc.AssertExpectations(t)
	locker.AssertExpectations(t)
	dpopSvc.AssertExpectations(t)
}

func TestRefreshHandler_Refresh_InsertTokenError(t *testing.T) {
	refresh := new(MockRefreshTokenService)
	jwtSvc := new(MockJwtService)
	locker := new(MockCache)
	dpopSvc := new(MockDPoPService)
	h := newRefreshHandler(refresh, jwtSvc, locker, dpopSvc)

	proof := makeProof(t)
	jkt := proofJkt(t, proof)
	lockKey := "refresh:lock:" + service.HashRefreshToken(testRefreshCookie)
	token := "new-access-token"

	dpopSvc.On("Validate", mock.Anything, mock.Anything).Return(proof, "nonce-1", nil).Once()
	refresh.On("GetRefreshToken", mock.Anything, testRefreshCookie).Return(&structs.RefreshTokenRecord{
		UserID: "u1", LineageID: "lin1", Jkt: jkt,
	}, nil).Once()
	locker.On("AcquireLock", mock.Anything, lockKey, mock.Anything, 5*time.Second).Return(true, nil).Once()
	refresh.On("ConsumeRefreshToken", mock.Anything, testRefreshCookie).Return(&structs.RefreshTokenRecord{
		UserID: "u1", LineageID: "lin1",
	}, nil).Once()
	jwtSvc.On("NewAccessToken", mock.Anything, "u1", domain.USER, jkt).Return(&token, nil).Once()
	refresh.On("InsertRefreshToken", mock.Anything, mock.Anything).Return(errs.ErrCacheNotFound).Once()
	locker.On("ReleaseLock", mock.Anything, lockKey, mock.Anything).Return(nil).Once()

	c, rec := refreshRequest(t, true)
	err := h.Refresh(c)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusNotFound, rec.Code)
	refresh.AssertExpectations(t)
	jwtSvc.AssertExpectations(t)
	locker.AssertExpectations(t)
	dpopSvc.AssertExpectations(t)
}

func TestRefreshHandler_Refresh_ConnError(t *testing.T) {
	refresh := new(MockRefreshTokenService)
	jwtSvc := new(MockJwtService)
	locker := new(MockCache)
	dpopSvc := new(MockDPoPService)
	h := newRefreshHandler(refresh, jwtSvc, locker, dpopSvc)

	proof := makeProof(t)
	dpopSvc.On("Validate", mock.Anything, mock.Anything).Return(proof, "nonce-1", nil).Once()
	// DB/Redis is down: GetRefreshToken fails with a connection error.
	refresh.On("GetRefreshToken", mock.Anything, testRefreshCookie).Return(nil, sql.ErrConnDone).Once()

	c, rec := refreshRequest(t, true)
	err := h.Refresh(c)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
	locker.AssertNotCalled(t, "AcquireLock", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
	refresh.AssertExpectations(t)
	dpopSvc.AssertExpectations(t)
}
