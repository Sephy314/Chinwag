package handler

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/Sephy314/chinwag/backend/services/auth/domain"
	"github.com/Sephy314/chinwag/backend/services/auth/shared/response"
	"github.com/Sephy314/chinwag/backend/services/auth/structs"
	"github.com/labstack/echo/v5"
	"github.com/labstack/echo/v5/echotest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestAdminSessionHandler_RevokeSession(t *testing.T) {
	cache := new(MockCache)
	audit := new(MockAuditRepo)
	h := newAdminSessionHandler(cache, audit)

	fields := map[string]string{"user_id": "u1"}
	cache.On("SMembers", mock.Anything, "refresh:lineage:lin1").Return([]string{"tok1"}, nil).Once()
	cache.On("HSet", mock.Anything, "refresh:tok1", map[string]string{"revoked": "1"}, mock.Anything).Return(nil).Once()
	cache.On("HGetAll", mock.Anything, "refresh:tok1").Return(fields, nil).Once()
	cache.On("ZRem", mock.Anything, "refresh:user:u1", []string{"lin1"}).Return(nil).Once()
	cache.On("ZRem", mock.Anything, "refresh:sessions", []string{"lin1"}).Return(nil).Once()
	cache.On("Delete", mock.Anything, "refresh:lineage:lin1").Return(nil).Once()
	audit.On("Insert", mock.Anything, mock.MatchedBy(func(ev domain.AuditEvent) bool {
		return ev.Action == "session.revoke" && ev.TargetId == "lin1"
	})).Return(nil).Once()

	c, rec := echotest.ContextConfig{
		PathValues: []echo.PathValue{{Name: "id", Value: "lin1"}},
	}.ToContextRecorder(t)
	setAdminClaims(c)

	err := h.RevokeSession(c)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)
	cache.AssertExpectations(t)
	audit.AssertExpectations(t)
}

func TestAdminSessionHandler_StatsSessions(t *testing.T) {
	cache := new(MockCache)
	h := newAdminSessionHandler(cache, new(MockAuditRepo))

	cache.On("ZCard", mock.Anything, "refresh:sessions").Return(int64(2), nil).Once()

	c, rec := echotest.ContextConfig{}.ToContextRecorder(t)
	err := h.StatsSessions(c)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)

	var resp response.Response[structs.StatsResponse]
	assert.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Equal(t, int64(2), resp.Data.Count)
	cache.AssertExpectations(t)
}
