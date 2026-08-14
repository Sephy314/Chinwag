package handler

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/Sephy314/chinwag/backend/services/auth/domain"
	"github.com/Sephy314/chinwag/backend/services/auth/shared/errs"
	"github.com/Sephy314/chinwag/backend/services/auth/shared/response"
	"github.com/Sephy314/chinwag/backend/services/auth/structs"
	"github.com/golang-jwt/jwt/v5"
	"github.com/labstack/echo/v5"
	"github.com/labstack/echo/v5/echotest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestAdminUserHandler_ListUsers_Success(t *testing.T) {
	userRepo := new(MockUserRepo)
	h := newAdminUserHandler(userRepo, new(MockCache), new(MockAuditRepo))

	users := []domain.User{{Id: "u1", Name: "Alice", Email: "a@example.com", Role: domain.USER, CreatedAt: time.Now()}}
	userRepo.On("ListUsers", mock.Anything, "", 50, "", "", "").Return(users, (*structs.CursorMeta)(nil), nil).Once()

	c, rec := echotest.ContextConfig{}.ToContextRecorder(t)
	err := h.ListUsers(c)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)

	var resp response.Response[[]structs.AdminUserResponse]
	assert.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Len(t, resp.Data, 1)
	assert.Equal(t, "u1", resp.Data[0].Id)
	userRepo.AssertExpectations(t)
}

func TestAdminUserHandler_CreateUser_Success(t *testing.T) {
	userRepo := new(MockUserRepo)
	audit := new(MockAuditRepo)
	h := newAdminUserHandler(userRepo, new(MockCache), audit)

	userRepo.On("CreateUser", mock.Anything, mock.MatchedBy(func(u domain.User) bool {
		return u.Name == "Bob" && u.Email == "b@example.com"
	})).Return(nil).Once()
	audit.On("Insert", mock.Anything, mock.MatchedBy(func(ev domain.AuditEvent) bool {
		return ev.Action == "user.create" && ev.AdminId == "admin1"
	})).Return(nil).Once()

	c, rec := echotest.ContextConfig{
		Headers:  map[string][]string{echo.HeaderContentType: {echo.MIMEApplicationJSON}},
		JSONBody: []byte(`{"name":"Bob","email":"b@example.com","password":"secret","role":"USER"}`),
	}.ToContextRecorder(t)
	setAdminClaims(c)

	err := h.CreateUser(c)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusCreated, rec.Code)
	userRepo.AssertExpectations(t)
	audit.AssertExpectations(t)
}

func TestAdminUserHandler_UpdateRole_SelfDemotion(t *testing.T) {
	userRepo := new(MockUserRepo)
	h := newAdminUserHandler(userRepo, new(MockCache), new(MockAuditRepo))

	// actor "admin1" demotes themselves -> rejected before any repo call.
	c, rec := echotest.ContextConfig{
		Headers:    map[string][]string{echo.HeaderContentType: {echo.MIMEApplicationJSON}},
		JSONBody:   []byte(`{"role":"USER"}`),
		PathValues: []echo.PathValue{{Name: "id", Value: "admin1"}},
	}.ToContextRecorder(t)
	setAdminClaims(c)

	err := h.UpdateRole(c)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	userRepo.AssertNotCalled(t, "SetRole", mock.Anything, mock.Anything, mock.Anything)
}

func TestAdminUserHandler_UpdateRole_LastAdmin(t *testing.T) {
	userRepo := new(MockUserRepo)
	h := newAdminUserHandler(userRepo, new(MockCache), new(MockAuditRepo))

	claims := &jwt.RegisteredClaims{Subject: "admin1"}
	_ = claims

	// Demoting another ADMIN when only 1 admin exists -> conflict.
	userRepo.On("GetUserIncludingDeleted", mock.Anything, "admin2").Return(&domain.User{Id: "admin2", Role: domain.ADMIN}, nil).Once()
	userRepo.On("CountAdmins", mock.Anything).Return(1, nil).Once()

	c, rec := echotest.ContextConfig{
		Headers:    map[string][]string{echo.HeaderContentType: {echo.MIMEApplicationJSON}},
		JSONBody:   []byte(`{"role":"USER"}`),
		PathValues: []echo.PathValue{{Name: "id", Value: "admin2"}},
	}.ToContextRecorder(t)
	setAdminClaims(c)

	err := h.UpdateRole(c)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusConflict, rec.Code)

	var resp response.Response[any]
	assert.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Equal(t, errs.ErrLastAdmin.Message, resp.Message)
	userRepo.AssertExpectations(t)
}

func TestAdminUserHandler_ListUserSessions(t *testing.T) {
	cache := new(MockCache)
	h := newAdminUserHandler(new(MockUserRepo), cache, new(MockAuditRepo))

	fields := map[string]string{
		"user_id": "u1", "sid": "lin1", "jkt": "jkt", "status": "active", "created_at": "1700000000",
	}
	cache.On("ZRevRangeByScore", mock.Anything, "refresh:user:u1", "+inf", "-inf", int64(0), int64(-1)).Return([]string{"lin1"}, nil).Once()
	cache.On("SMembers", mock.Anything, "refresh:lineage:lin1:members").Return([]string{"tok1"}, nil).Once()
	cache.On("HGetAll", mock.Anything, "refresh:tok1").Return(fields, nil).Once()

	c, rec := echotest.ContextConfig{
		PathValues: []echo.PathValue{{Name: "id", Value: "u1"}},
	}.ToContextRecorder(t)

	err := h.ListUserSessions(c)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)

	var resp response.Response[[]structs.AdminSession]
	assert.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Len(t, resp.Data, 1)
	assert.Equal(t, "lin1", resp.Data[0].LineageId)
	cache.AssertExpectations(t)
}
