package handler

import (
	"database/sql"
	"encoding/json"
	"errors"
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
	userSvc := service.NewUserService(new(MockUserRepo), new(MockJwksService), new(MockRefreshTokenService), &noopLogger{})
	return NewRefreshHandler(refresh, jwtSvc, locker, dpopSvc, userSvc, logger.New())
}

// newRefreshHandlerWithUser builds the handler with a caller-provided user
// repo so tests can stub GetUser and assert the refreshed role.
func newRefreshHandlerWithUser(refresh *MockRefreshTokenService, jwtSvc *MockJwtService, locker *MockCache, dpopSvc *MockDPoPService, userRepo *MockUserRepo) *RefreshHandlerImpl {
	userSvc := service.NewUserService(userRepo, new(MockJwksService), new(MockRefreshTokenService), &noopLogger{})
	return NewRefreshHandler(refresh, jwtSvc, locker, dpopSvc, userSvc, logger.New())
}

func TestRefreshHandler_Refresh_Success(t *testing.T) {
	refresh := new(MockRefreshTokenService)
	jwtSvc := new(MockJwtService)
	locker := new(MockCache)
	dpopSvc := new(MockDPoPService)
	userRepo := new(MockUserRepo)
	h := newRefreshHandlerWithUser(refresh, jwtSvc, locker, dpopSvc, userRepo)

	proof := makeProof(t)
	jkt := proofJkt(t, proof)
	lockKey := "refresh:lock:" + service.HashRefreshToken(testRefreshCookie)
	token := "new-access-token"

	dpopSvc.On("Validate", mock.Anything, mock.Anything).Return(proof, "nonce-1", nil).Once()
	refresh.On("ValidateRefreshToken", mock.Anything, testRefreshCookie, jkt).Return(&structs.RefreshTokenClaims{
		Subject: "u1", JTI: "jti1", SID: "lin1", Jkt: jkt,
	}, nil).Once()
	locker.On("AcquireLock", mock.Anything, lockKey, mock.Anything, 5*time.Second).Return(true, nil).Once()
	refresh.On("RotateRefreshToken", mock.Anything, mock.MatchedBy(func(c *structs.RefreshTokenClaims) bool {
		return c.Subject == "u1" && c.JTI == "jti1" && c.SID == "lin1"
	})).Return(&structs.RotatedRefreshToken{
		NewToken: "new-refresh-token", NewJTI: "jti2", SID: "lin1", UserID: "u1",
	}, nil).Once()
	// The fresh access token must carry the user's CURRENT role (ADMIN), not a
	// hardcoded USER — this is what lets admins keep their role across refreshes.
	userRepo.On("GetUser", mock.Anything, "u1").Return(&domain.User{Id: "u1", Role: domain.ADMIN}, nil).Once()
	jwtSvc.On("NewAccessToken", mock.Anything, "u1", domain.ADMIN, jkt).Return(&token, nil).Once()
	locker.On("ReleaseLock", mock.Anything, lockKey, mock.Anything).Return(nil).Once()

	c, rec := refreshRequest(t, true)
	err := h.Refresh(c)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)

	var resp response.Response[structs.LoginUserResp]
	assert.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Equal(t, token, resp.Data.Token)

	// the rotated refresh token is returned to the browser as a cookie
	gotCookie := rec.Result().Cookies()
	var refreshCookie *http.Cookie
	for _, ck := range gotCookie {
		if ck.Name == "refresh" {
			refreshCookie = ck
		}
	}
	if assert.NotNil(t, refreshCookie) {
		assert.Equal(t, "new-refresh-token", refreshCookie.Value)
	}
	refresh.AssertExpectations(t)
	jwtSvc.AssertExpectations(t)
	locker.AssertExpectations(t)
	dpopSvc.AssertExpectations(t)
	userRepo.AssertExpectations(t)
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
	refresh.AssertNotCalled(t, "ValidateRefreshToken", mock.Anything, mock.Anything, mock.Anything)
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
	refresh.AssertNotCalled(t, "ValidateRefreshToken", mock.Anything, mock.Anything, mock.Anything)
	dpopSvc.AssertExpectations(t)
}

// Redis is down: the DPoP nonce store fails with a plain (non-*dpop.Error)
// error, which must surface as a 503 dependency failure — NOT an invalid-proof
// 400. This is the degraded-mode behaviour.
func TestRefreshHandler_Refresh_DPoPDependency(t *testing.T) {
	refresh := new(MockRefreshTokenService)
	jwtSvc := new(MockJwtService)
	locker := new(MockCache)
	dpopSvc := new(MockDPoPService)
	h := newRefreshHandler(refresh, jwtSvc, locker, dpopSvc)

	dpopSvc.On("Validate", mock.Anything, mock.Anything).Return(nil, "", errors.New("redis down")).Once()

	c, rec := refreshRequest(t, true)
	err := h.Refresh(c)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusServiceUnavailable, rec.Code)
	dpopSvc.AssertExpectations(t)
}

func TestRefreshHandler_Refresh_InvalidToken(t *testing.T) {
	refresh := new(MockRefreshTokenService)
	jwtSvc := new(MockJwtService)
	locker := new(MockCache)
	dpopSvc := new(MockDPoPService)
	h := newRefreshHandler(refresh, jwtSvc, locker, dpopSvc)

	proof := makeProof(t)
	jkt := proofJkt(t, proof)
	dpopSvc.On("Validate", mock.Anything, mock.Anything).Return(proof, "nonce-1", nil).Once()
	refresh.On("ValidateRefreshToken", mock.Anything, testRefreshCookie, jkt).Return(nil, errs.ErrInvalidRefreshToken).Once()

	c, rec := refreshRequest(t, true)
	err := h.Refresh(c)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
	locker.AssertNotCalled(t, "AcquireLock", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
	refresh.AssertExpectations(t)
	dpopSvc.AssertExpectations(t)
}

func TestRefreshHandler_Refresh_BindingMismatch(t *testing.T) {
	refresh := new(MockRefreshTokenService)
	jwtSvc := new(MockJwtService)
	locker := new(MockCache)
	dpopSvc := new(MockDPoPService)
	h := newRefreshHandler(refresh, jwtSvc, locker, dpopSvc)

	proof := makeProof(t)
	jkt := proofJkt(t, proof)
	dpopSvc.On("Validate", mock.Anything, mock.Anything).Return(proof, "nonce-1", nil).Once()
	refresh.On("ValidateRefreshToken", mock.Anything, testRefreshCookie, jkt).Return(nil, errs.ErrRefreshTokenBindingMismatch).Once()

	c, rec := refreshRequest(t, true)
	err := h.Refresh(c)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
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
	refresh.On("ValidateRefreshToken", mock.Anything, testRefreshCookie, jkt).Return(&structs.RefreshTokenClaims{
		Subject: "u1", JTI: "jti1", SID: "lin1", Jkt: jkt,
	}, nil).Once()
	locker.On("AcquireLock", mock.Anything, lockKey, mock.Anything, 5*time.Second).Return(false, nil).Once()

	c, rec := refreshRequest(t, true)
	err := h.Refresh(c)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusTooManyRequests, rec.Code)
	refresh.AssertNotCalled(t, "RotateRefreshToken", mock.Anything, mock.Anything)
	refresh.AssertExpectations(t)
	locker.AssertExpectations(t)
	dpopSvc.AssertExpectations(t)
}

// Redis is down at the lock step: no rotation is attempted and a 503 is
// returned (degraded mode — the token is NOT consumed).
func TestRefreshHandler_Refresh_LockDependency(t *testing.T) {
	refresh := new(MockRefreshTokenService)
	jwtSvc := new(MockJwtService)
	locker := new(MockCache)
	dpopSvc := new(MockDPoPService)
	h := newRefreshHandler(refresh, jwtSvc, locker, dpopSvc)

	proof := makeProof(t)
	jkt := proofJkt(t, proof)
	lockKey := "refresh:lock:" + service.HashRefreshToken(testRefreshCookie)

	dpopSvc.On("Validate", mock.Anything, mock.Anything).Return(proof, "nonce-1", nil).Once()
	refresh.On("ValidateRefreshToken", mock.Anything, testRefreshCookie, jkt).Return(&structs.RefreshTokenClaims{
		Subject: "u1", JTI: "jti1", SID: "lin1", Jkt: jkt,
	}, nil).Once()
	locker.On("AcquireLock", mock.Anything, lockKey, mock.Anything, 5*time.Second).Return(false, errors.New("redis down")).Once()

	c, rec := refreshRequest(t, true)
	err := h.Refresh(c)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusServiceUnavailable, rec.Code)
	refresh.AssertNotCalled(t, "RotateRefreshToken", mock.Anything, mock.Anything)
	refresh.AssertExpectations(t)
	locker.AssertExpectations(t)
	dpopSvc.AssertExpectations(t)
}

// Redis is down during rotation: the atomic script did NOT run (no state
// change), so the RT is untouched and the client may retry.
func TestRefreshHandler_Refresh_RotationDependency(t *testing.T) {
	refresh := new(MockRefreshTokenService)
	jwtSvc := new(MockJwtService)
	locker := new(MockCache)
	dpopSvc := new(MockDPoPService)
	h := newRefreshHandler(refresh, jwtSvc, locker, dpopSvc)

	proof := makeProof(t)
	jkt := proofJkt(t, proof)
	lockKey := "refresh:lock:" + service.HashRefreshToken(testRefreshCookie)

	dpopSvc.On("Validate", mock.Anything, mock.Anything).Return(proof, "nonce-1", nil).Once()
	refresh.On("ValidateRefreshToken", mock.Anything, testRefreshCookie, jkt).Return(&structs.RefreshTokenClaims{
		Subject: "u1", JTI: "jti1", SID: "lin1", Jkt: jkt,
	}, nil).Once()
	locker.On("AcquireLock", mock.Anything, lockKey, mock.Anything, 5*time.Second).Return(true, nil).Once()
	refresh.On("RotateRefreshToken", mock.Anything, mock.Anything).Return(nil, errs.ErrDependencyUnavailable).Once()
	locker.On("ReleaseLock", mock.Anything, lockKey, mock.Anything).Return(nil).Once()

	c, rec := refreshRequest(t, true)
	err := h.Refresh(c)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusServiceUnavailable, rec.Code)
	userRepo := h.users
	_ = userRepo
	jwtSvc.AssertNotCalled(t, "NewAccessToken", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
	refresh.AssertExpectations(t)
	locker.AssertExpectations(t)
	dpopSvc.AssertExpectations(t)
}

// Reuse of an already-rotated RT: the atomic rotation script revokes the whole
// lineage in the same operation (RFC 9700), so the handler must NOT perform a
// separate revocation — it only surfaces the reuse error.
func TestRefreshHandler_Refresh_ReusedTokenAtomicallyRevokesLineage(t *testing.T) {
	refresh := new(MockRefreshTokenService)
	jwtSvc := new(MockJwtService)
	locker := new(MockCache)
	dpopSvc := new(MockDPoPService)
	h := newRefreshHandler(refresh, jwtSvc, locker, dpopSvc)

	proof := makeProof(t)
	jkt := proofJkt(t, proof)
	lockKey := "refresh:lock:" + service.HashRefreshToken(testRefreshCookie)

	dpopSvc.On("Validate", mock.Anything, mock.Anything).Return(proof, "nonce-1", nil).Once()
	refresh.On("ValidateRefreshToken", mock.Anything, testRefreshCookie, jkt).Return(&structs.RefreshTokenClaims{
		Subject: "u1", JTI: "jti1", SID: "lin1", Jkt: jkt,
	}, nil).Once()
	locker.On("AcquireLock", mock.Anything, lockKey, mock.Anything, 5*time.Second).Return(true, nil).Once()
	refresh.On("RotateRefreshToken", mock.Anything, mock.Anything).Return(nil, errs.ErrRefreshTokenReused).Once()
	locker.On("ReleaseLock", mock.Anything, lockKey, mock.Anything).Return(nil).Once()

	c, rec := refreshRequest(t, true)
	err := h.Refresh(c)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
	// Lineage revocation is handled inside the atomic rotation script; the
	// handler must not issue a separate RevokeLineage call.
	refresh.AssertNotCalled(t, "RevokeLineage", mock.Anything, mock.Anything)
	jwtSvc.AssertNotCalled(t, "NewAccessToken", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
	refresh.AssertExpectations(t)
	locker.AssertExpectations(t)
	dpopSvc.AssertExpectations(t)
}

// A lineage that was already revoked (admin revoke, or a previous reuse) is
// rejected without a new token.
func TestRefreshHandler_Refresh_LineageRevoked(t *testing.T) {
	refresh := new(MockRefreshTokenService)
	jwtSvc := new(MockJwtService)
	locker := new(MockCache)
	dpopSvc := new(MockDPoPService)
	h := newRefreshHandler(refresh, jwtSvc, locker, dpopSvc)

	proof := makeProof(t)
	jkt := proofJkt(t, proof)
	lockKey := "refresh:lock:" + service.HashRefreshToken(testRefreshCookie)

	dpopSvc.On("Validate", mock.Anything, mock.Anything).Return(proof, "nonce-1", nil).Once()
	refresh.On("ValidateRefreshToken", mock.Anything, testRefreshCookie, jkt).Return(&structs.RefreshTokenClaims{
		Subject: "u1", JTI: "jti1", SID: "lin1", Jkt: jkt,
	}, nil).Once()
	locker.On("AcquireLock", mock.Anything, lockKey, mock.Anything, 5*time.Second).Return(true, nil).Once()
	refresh.On("RotateRefreshToken", mock.Anything, mock.Anything).Return(nil, errs.ErrRefreshTokenRevoked).Once()
	locker.On("ReleaseLock", mock.Anything, lockKey, mock.Anything).Return(nil).Once()

	c, rec := refreshRequest(t, true)
	err := h.Refresh(c)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
	jwtSvc.AssertNotCalled(t, "NewAccessToken", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
	refresh.AssertExpectations(t)
	locker.AssertExpectations(t)
	dpopSvc.AssertExpectations(t)
}

func TestRefreshHandler_Refresh_DisabledUserRejected(t *testing.T) {
	refresh := new(MockRefreshTokenService)
	jwtSvc := new(MockJwtService)
	locker := new(MockCache)
	dpopSvc := new(MockDPoPService)
	userRepo := new(MockUserRepo)
	h := newRefreshHandlerWithUser(refresh, jwtSvc, locker, dpopSvc, userRepo)

	proof := makeProof(t)
	jkt := proofJkt(t, proof)
	lockKey := "refresh:lock:" + service.HashRefreshToken(testRefreshCookie)

	dpopSvc.On("Validate", mock.Anything, mock.Anything).Return(proof, "nonce-1", nil).Once()
	refresh.On("ValidateRefreshToken", mock.Anything, testRefreshCookie, jkt).Return(&structs.RefreshTokenClaims{
		Subject: "u1", JTI: "jti1", SID: "lin1", Jkt: jkt,
	}, nil).Once()
	locker.On("AcquireLock", mock.Anything, lockKey, mock.Anything, 5*time.Second).Return(true, nil).Once()
	refresh.On("RotateRefreshToken", mock.Anything, mock.Anything).Return(&structs.RotatedRefreshToken{
		NewToken: "new-refresh-token", NewJTI: "jti2", SID: "lin1", UserID: "u1",
	}, nil).Once()
	// Soft-deleted users are excluded by GetUser -> refresh is rejected and no
	// new access token is minted.
	userRepo.On("GetUser", mock.Anything, "u1").Return(nil, sql.ErrNoRows).Once()
	locker.On("ReleaseLock", mock.Anything, lockKey, mock.Anything).Return(nil).Once()

	c, rec := refreshRequest(t, true)
	err := h.Refresh(c)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
	jwtSvc.AssertNotCalled(t, "NewAccessToken", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
	refresh.AssertExpectations(t)
	locker.AssertExpectations(t)
	dpopSvc.AssertExpectations(t)
	userRepo.AssertExpectations(t)
}

func TestRefreshHandler_Refresh_NewAccessTokenError(t *testing.T) {
	refresh := new(MockRefreshTokenService)
	jwtSvc := new(MockJwtService)
	locker := new(MockCache)
	dpopSvc := new(MockDPoPService)
	userRepo := new(MockUserRepo)
	h := newRefreshHandlerWithUser(refresh, jwtSvc, locker, dpopSvc, userRepo)

	proof := makeProof(t)
	jkt := proofJkt(t, proof)
	lockKey := "refresh:lock:" + service.HashRefreshToken(testRefreshCookie)

	dpopSvc.On("Validate", mock.Anything, mock.Anything).Return(proof, "nonce-1", nil).Once()
	refresh.On("ValidateRefreshToken", mock.Anything, testRefreshCookie, jkt).Return(&structs.RefreshTokenClaims{
		Subject: "u1", JTI: "jti1", SID: "lin1", Jkt: jkt,
	}, nil).Once()
	locker.On("AcquireLock", mock.Anything, lockKey, mock.Anything, 5*time.Second).Return(true, nil).Once()
	refresh.On("RotateRefreshToken", mock.Anything, mock.Anything).Return(&structs.RotatedRefreshToken{
		NewToken: "new-refresh-token", NewJTI: "jti2", SID: "lin1", UserID: "u1",
	}, nil).Once()
	userRepo.On("GetUser", mock.Anything, "u1").Return(&domain.User{Id: "u1", Role: domain.MANAGER}, nil).Once()
	jwtSvc.On("NewAccessToken", mock.Anything, "u1", domain.MANAGER, jkt).Return(nil, errs.ErrNoKey).Once()
	locker.On("ReleaseLock", mock.Anything, lockKey, mock.Anything).Return(nil).Once()

	c, rec := refreshRequest(t, true)
	err := h.Refresh(c)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	refresh.AssertExpectations(t)
	jwtSvc.AssertExpectations(t)
	locker.AssertExpectations(t)
	dpopSvc.AssertExpectations(t)
	userRepo.AssertExpectations(t)
}
