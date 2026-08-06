package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Sephy314/chinwag/backend/services/auth/shared/errs"
	"github.com/Sephy314/chinwag/backend/services/auth/structs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func newRefreshSvc(cache *MockCache) *RefreshTokenService {
	return NewRefreshTokenService(cache, "rt:", time.Hour)
}

func TestRefreshTokenService_GetRefreshToken_Success(t *testing.T) {
	cache := new(MockCache)
	svc := newRefreshSvc(cache)

	fields := map[string]string{
		"user_id":     "u1",
		"lineage_id":  "lin1",
		"parent_hash": "ph",
		"jkt":         "jkt",
		"used":        "1",
		"revoked":     "0",
		"created_at":  "1700000000",
	}
	cache.On("HGetAll", mock.Anything, "rt:tok123").Return(fields, nil).Once()

	rec, err := svc.GetRefreshToken(context.Background(), "tok123")

	assert.NoError(t, err)
	assert.NotNil(t, rec)
	assert.Equal(t, "u1", rec.UserID)
	assert.Equal(t, "lin1", rec.LineageID)
	assert.True(t, rec.Used)
	assert.False(t, rec.Revoked)
	assert.Equal(t, int64(1700000000), rec.CreatedAt)
	cache.AssertExpectations(t)
}

func TestRefreshTokenService_GetRefreshToken_NotFound(t *testing.T) {
	cache := new(MockCache)
	svc := newRefreshSvc(cache)

	cache.On("HGetAll", mock.Anything, "rt:tok123").Return(map[string]string{}, nil).Once()

	rec, err := svc.GetRefreshToken(context.Background(), "tok123")

	assert.Nil(t, rec)
	assert.ErrorIs(t, err, errs.ErrCacheNotFound)
	cache.AssertExpectations(t)
}

func TestRefreshTokenService_GetRefreshToken_CacheError(t *testing.T) {
	cache := new(MockCache)
	svc := newRefreshSvc(cache)

	cache.On("HGetAll", mock.Anything, "rt:tok123").Return(nil, errors.New("redis down")).Once()

	rec, err := svc.GetRefreshToken(context.Background(), "tok123")

	assert.Nil(t, rec)
	assert.EqualError(t, err, "redis down")
	cache.AssertExpectations(t)
}

func TestRefreshTokenService_InsertRefreshToken_Success_WithLineage(t *testing.T) {
	cache := new(MockCache)
	svc := newRefreshSvc(cache)

	token := structs.RefreshToken{
		Subject:      "u1",
		RefreshToken: "tok123",
		LineageID:    "lin1",
		ParentHash:   "ph",
		Jkt:          "jkt",
	}

	cache.On("HSet", mock.Anything, "rt:tok123", mock.MatchedBy(func(f map[string]string) bool {
		return f["user_id"] == "u1" && f["lineage_id"] == "lin1" && f["used"] == "0" && f["revoked"] == "0"
	}), time.Hour).Return(nil).Once()
	cache.On("SAdd", mock.Anything, "rt:lineage:lin1", time.Hour, []string{"tok123"}).Return(nil).Once()

	err := svc.InsertRefreshToken(context.Background(), token)

	assert.NoError(t, err)
	cache.AssertExpectations(t)
}

func TestRefreshTokenService_InsertRefreshToken_AutoLineage(t *testing.T) {
	cache := new(MockCache)
	svc := newRefreshSvc(cache)

	token := structs.RefreshToken{Subject: "u1", RefreshToken: "tok123"}

	cache.On("HSet", mock.Anything, "rt:tok123", mock.MatchedBy(func(f map[string]string) bool {
		return f["user_id"] == "u1" && f["lineage_id"] != ""
	}), time.Hour).Return(nil).Once()
	cache.On("SAdd", mock.Anything, mock.MatchedBy(func(k string) bool {
		return len(k) > len("rt:lineage:")
	}), time.Hour, []string{"tok123"}).Return(nil).Once()

	err := svc.InsertRefreshToken(context.Background(), token)

	assert.NoError(t, err)
	cache.AssertExpectations(t)
}

func TestRefreshTokenService_InsertRefreshToken_HSetError(t *testing.T) {
	cache := new(MockCache)
	svc := newRefreshSvc(cache)

	token := structs.RefreshToken{Subject: "u1", RefreshToken: "tok123"}
	cache.On("HSet", mock.Anything, "rt:tok123", mock.Anything, time.Hour).Return(errors.New("redis down")).Once()

	err := svc.InsertRefreshToken(context.Background(), token)

	assert.EqualError(t, err, "redis down")
	cache.AssertNotCalled(t, "SAdd", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
	cache.AssertExpectations(t)
}

func TestRefreshTokenService_InsertRefreshToken_SAddError(t *testing.T) {
	cache := new(MockCache)
	svc := newRefreshSvc(cache)

	token := structs.RefreshToken{Subject: "u1", RefreshToken: "tok123"}
	cache.On("HSet", mock.Anything, "rt:tok123", mock.Anything, time.Hour).Return(nil).Once()
	cache.On("SAdd", mock.Anything, mock.Anything, time.Hour, mock.Anything).Return(errors.New("redis down")).Once()

	err := svc.InsertRefreshToken(context.Background(), token)

	assert.EqualError(t, err, "redis down")
	cache.AssertExpectations(t)
}

func TestRefreshTokenService_ConsumeRefreshToken_OK(t *testing.T) {
	cache := new(MockCache)
	svc := newRefreshSvc(cache)

	result := []interface{}{"OK", "u1", "lin1", "ph", "jkt"}
	cache.On("Eval", mock.Anything, mock.Anything, []string{"rt:tok123"}, mock.Anything).Return(result, nil).Once()

	rec, err := svc.ConsumeRefreshToken(context.Background(), "tok123")

	assert.NoError(t, err)
	assert.NotNil(t, rec)
	assert.Equal(t, "u1", rec.UserID)
	assert.Equal(t, "lin1", rec.LineageID)
	assert.False(t, rec.Used)
	assert.False(t, rec.Revoked)
	cache.AssertExpectations(t)
}

func TestRefreshTokenService_ConsumeRefreshToken_NotFound(t *testing.T) {
	cache := new(MockCache)
	svc := newRefreshSvc(cache)

	result := []interface{}{"NOT_FOUND", "", "", "", ""}
	cache.On("Eval", mock.Anything, mock.Anything, []string{"rt:tok123"}, mock.Anything).Return(result, nil).Once()

	rec, err := svc.ConsumeRefreshToken(context.Background(), "tok123")

	assert.Nil(t, rec)
	assert.ErrorIs(t, err, errs.ErrCacheNotFound)
	cache.AssertExpectations(t)
}

func TestRefreshTokenService_ConsumeRefreshToken_Revoked(t *testing.T) {
	cache := new(MockCache)
	svc := newRefreshSvc(cache)

	result := []interface{}{"REVOKED", "u1", "lin1", "ph", "jkt"}
	cache.On("Eval", mock.Anything, mock.Anything, []string{"rt:tok123"}, mock.Anything).Return(result, nil).Once()

	rec, err := svc.ConsumeRefreshToken(context.Background(), "tok123")

	assert.NotNil(t, rec)
	assert.True(t, rec.Revoked)
	assert.ErrorIs(t, err, errs.ErrRefreshTokenRevoked)
	cache.AssertExpectations(t)
}

func TestRefreshTokenService_ConsumeRefreshToken_Reused(t *testing.T) {
	cache := new(MockCache)
	svc := newRefreshSvc(cache)

	result := []interface{}{"USED", "u1", "lin1", "ph", "jkt"}
	cache.On("Eval", mock.Anything, mock.Anything, []string{"rt:tok123"}, mock.Anything).Return(result, nil).Once()

	rec, err := svc.ConsumeRefreshToken(context.Background(), "tok123")

	assert.NotNil(t, rec)
	assert.True(t, rec.Used)
	assert.ErrorIs(t, err, errs.ErrRefreshTokenReused)
	cache.AssertExpectations(t)
}

func TestRefreshTokenService_ConsumeRefreshToken_InvalidResult(t *testing.T) {
	cache := new(MockCache)
	svc := newRefreshSvc(cache)

	// not an array
	cache.On("Eval", mock.Anything, mock.Anything, []string{"rt:tok123"}, mock.Anything).Return("unexpected", nil).Once()

	rec, err := svc.ConsumeRefreshToken(context.Background(), "tok123")

	assert.Nil(t, rec)
	assert.ErrorIs(t, err, errs.ErrCacheNotFound)
	cache.AssertExpectations(t)
}

func TestRefreshTokenService_ConsumeRefreshToken_EvalError(t *testing.T) {
	cache := new(MockCache)
	svc := newRefreshSvc(cache)

	cache.On("Eval", mock.Anything, mock.Anything, []string{"rt:tok123"}, mock.Anything).Return(nil, errors.New("redis down")).Once()

	rec, err := svc.ConsumeRefreshToken(context.Background(), "tok123")

	assert.Nil(t, rec)
	assert.EqualError(t, err, "redis down")
	cache.AssertExpectations(t)
}

func TestRefreshTokenService_RevokeLineage_Empty(t *testing.T) {
	cache := new(MockCache)
	svc := newRefreshSvc(cache)

	err := svc.RevokeLineage(context.Background(), "")

	assert.NoError(t, err)
	cache.AssertNotCalled(t, "SMembers", mock.Anything, mock.Anything)
}

func TestRefreshTokenService_RevokeLineage_Success(t *testing.T) {
	cache := new(MockCache)
	svc := newRefreshSvc(cache)

	cache.On("SMembers", mock.Anything, "rt:lineage:lin1").Return([]string{"tok1", "tok2"}, nil).Once()
	cache.On("HSet", mock.Anything, "rt:tok1", map[string]string{"revoked": "1"}, time.Duration(0)).Return(nil).Once()
	cache.On("HSet", mock.Anything, "rt:tok2", map[string]string{"revoked": "1"}, time.Duration(0)).Return(nil).Once()
	cache.On("Delete", mock.Anything, "rt:lineage:lin1").Return(nil).Once()

	err := svc.RevokeLineage(context.Background(), "lin1")

	assert.NoError(t, err)
	cache.AssertExpectations(t)
}

func TestRefreshTokenService_RevokeLineage_SMembersError(t *testing.T) {
	cache := new(MockCache)
	svc := newRefreshSvc(cache)

	cache.On("SMembers", mock.Anything, "rt:lineage:lin1").Return(nil, errors.New("redis down")).Once()

	err := svc.RevokeLineage(context.Background(), "lin1")

	assert.EqualError(t, err, "redis down")
	cache.AssertNotCalled(t, "Delete", mock.Anything, mock.Anything)
	cache.AssertExpectations(t)
}

func TestRefreshTokenService_RevokeLineage_HSetError(t *testing.T) {
	cache := new(MockCache)
	svc := newRefreshSvc(cache)

	cache.On("SMembers", mock.Anything, "rt:lineage:lin1").Return([]string{"tok1"}, nil).Once()
	cache.On("HSet", mock.Anything, "rt:tok1", map[string]string{"revoked": "1"}, time.Duration(0)).Return(errors.New("redis down")).Once()

	err := svc.RevokeLineage(context.Background(), "lin1")

	assert.EqualError(t, err, "redis down")
	cache.AssertNotCalled(t, "Delete", mock.Anything, mock.Anything)
	cache.AssertExpectations(t)
}

func TestHashRefreshToken_Stable(t *testing.T) {
	h1 := HashRefreshToken("abc")
	h2 := HashRefreshToken("abc")
	h3 := HashRefreshToken("abd")

	assert.Equal(t, h1, h2)
	assert.NotEqual(t, h1, h3)
	assert.Len(t, h1, 64)
}
