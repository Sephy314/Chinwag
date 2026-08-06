package handler

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/Sephy314/chinwag/backend/services/room/domain"
	"github.com/Sephy314/chinwag/backend/services/room/shared/errs"
	"github.com/Sephy314/chinwag/backend/services/room/shared/response"
	"github.com/Sephy314/chinwag/backend/services/room/structs"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/labstack/echo/v5"
	"github.com/labstack/echo/v5/echotest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func setUserToken(c *echo.Context, userId uuid.UUID) {
	c.Set("user", &jwt.Token{Claims: &jwt.RegisteredClaims{Subject: userId.String()}})
}

func TestRoomHandler_Health(t *testing.T) {
	h := NewRoomHandler(new(MockRoomService), new(MockRoomMemberService))

	c, rec := echotest.ContextConfig{}.ToContextRecorder(t)
	err := h.Health(c)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)

	var resp response.Response[any]
	assert.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.True(t, resp.Success)
}

func TestRoomHandler_CreateRoom_Success(t *testing.T) {
	svc := new(MockRoomService)
	h := NewRoomHandler(svc, new(MockRoomMemberService))

	ownerId := uuid.New()
	roomId := uuid.New()
	now := time.Now()

	expected := &domain.Room{Id: roomId, Name: "General", MaxMembers: 10, OwnerId: ownerId, CreatedAt: now}
	svc.On("CreateRoom", mock.Anything, mock.MatchedBy(func(req structs.CreateRoomRequest) bool {
		return req.Name == "General" && req.MaxMembers == 10
	})).Return(expected, nil).Once()

	c, rec := echotest.ContextConfig{
		Headers: map[string][]string{
			echo.HeaderContentType: {echo.MIMEApplicationJSON},
		},
		JSONBody: []byte(`{"name":"General","max_members":10}`),
	}.ToContextRecorder(t)
	setUserToken(c, ownerId)

	err := h.CreateRoom(c)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusCreated, rec.Code)

	var resp response.Response[domain.Room]
	assert.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.True(t, resp.Success)
	assert.Equal(t, "General", resp.Data.Name)
	svc.AssertExpectations(t)
}

func TestRoomHandler_CreateRoom_InvalidBody(t *testing.T) {
	svc := new(MockRoomService)
	h := NewRoomHandler(svc, new(MockRoomMemberService))

	c, rec := echotest.ContextConfig{
		Headers: map[string][]string{
			echo.HeaderContentType: {echo.MIMEApplicationJSON},
		},
		JSONBody: []byte(`{invalid`),
	}.ToContextRecorder(t)
	setUserToken(c, uuid.New())

	err := h.CreateRoom(c)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	svc.AssertNotCalled(t, "CreateRoom", mock.Anything, mock.Anything)
}

func TestRoomHandler_CreateRoom_Unauthorized(t *testing.T) {
	svc := new(MockRoomService)
	h := NewRoomHandler(svc, new(MockRoomMemberService))

	c, _ := echotest.ContextConfig{
		Headers: map[string][]string{
			echo.HeaderContentType: {echo.MIMEApplicationJSON},
		},
		JSONBody: []byte(`{"name":"General"}`),
	}.ToContextRecorder(t)
	// no user token set -> handler returns echo.ErrUnauthorized

	err := h.CreateRoom(c)
	assert.ErrorIs(t, err, echo.ErrUnauthorized)
	svc.AssertNotCalled(t, "CreateRoom", mock.Anything, mock.Anything)
}

func TestRoomHandler_CreateRoom_ServiceError(t *testing.T) {
	svc := new(MockRoomService)
	h := NewRoomHandler(svc, new(MockRoomMemberService))

	ownerId := uuid.New()
	svc.On("CreateRoom", mock.Anything, mock.Anything).Return(nil, errs.ErrConflict).Once()

	c, rec := echotest.ContextConfig{
		Headers: map[string][]string{
			echo.HeaderContentType: {echo.MIMEApplicationJSON},
		},
		JSONBody: []byte(`{"name":"General"}`),
	}.ToContextRecorder(t)
	setUserToken(c, ownerId)

	err := h.CreateRoom(c)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusConflict, rec.Code)
	svc.AssertExpectations(t)
}

func TestRoomHandler_GetRoom_Success(t *testing.T) {
	svc := new(MockRoomService)
	h := NewRoomHandler(svc, new(MockRoomMemberService))

	roomId := uuid.New()
	expected := &domain.Room{Id: roomId, Name: "General"}
	svc.On("GetRoomById", mock.Anything, roomId).Return(expected, nil).Once()

	c, rec := echotest.ContextConfig{
		PathValues: []echo.PathValue{{Name: "id", Value: roomId.String()}},
	}.ToContextRecorder(t)

	err := h.GetRoom(c)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)

	var resp response.Response[domain.Room]
	assert.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Equal(t, roomId, resp.Data.Id)
	svc.AssertExpectations(t)
}

func TestRoomHandler_GetRoom_InvalidID(t *testing.T) {
	svc := new(MockRoomService)
	h := NewRoomHandler(svc, new(MockRoomMemberService))

	c, rec := echotest.ContextConfig{
		PathValues: []echo.PathValue{{Name: "id", Value: "not-a-uuid"}},
	}.ToContextRecorder(t)

	err := h.GetRoom(c)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	svc.AssertNotCalled(t, "GetRoomById", mock.Anything, mock.Anything)
}

func TestRoomHandler_GetRoom_NotFound(t *testing.T) {
	svc := new(MockRoomService)
	h := NewRoomHandler(svc, new(MockRoomMemberService))

	roomId := uuid.New()
	svc.On("GetRoomById", mock.Anything, roomId).Return(nil, errs.ErrNotFound).Once()

	c, rec := echotest.ContextConfig{
		PathValues: []echo.PathValue{{Name: "id", Value: roomId.String()}},
	}.ToContextRecorder(t)

	err := h.GetRoom(c)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusNotFound, rec.Code)
	svc.AssertExpectations(t)
}

func TestRoomHandler_ListRoomsByOwnerId_Success(t *testing.T) {
	svc := new(MockRoomService)
	h := NewRoomHandler(svc, new(MockRoomMemberService))

	ownerId := uuid.New()
	expected := []domain.Room{{Id: uuid.New(), Name: "A"}}
	svc.On("GetRoomsByOwnerId", mock.Anything, ownerId).Return(expected, nil).Once()

	c, rec := echotest.ContextConfig{
		PathValues: []echo.PathValue{{Name: "ownerId", Value: ownerId.String()}},
	}.ToContextRecorder(t)

	err := h.ListRoomsByOwnerId(c)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)

	var resp response.Response[[]domain.Room]
	assert.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Len(t, resp.Data, 1)
	svc.AssertExpectations(t)
}

func TestRoomHandler_ListRoomsByOwnerId_InvalidID(t *testing.T) {
	svc := new(MockRoomService)
	h := NewRoomHandler(svc, new(MockRoomMemberService))

	c, rec := echotest.ContextConfig{
		PathValues: []echo.PathValue{{Name: "ownerId", Value: "bad"}},
	}.ToContextRecorder(t)

	err := h.ListRoomsByOwnerId(c)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	svc.AssertNotCalled(t, "GetRoomsByOwnerId", mock.Anything, mock.Anything)
}

func TestRoomHandler_ListRoomsByMemberId_Success(t *testing.T) {
	memberSvc := new(MockRoomMemberService)
	h := NewRoomHandler(new(MockRoomService), memberSvc)

	memberId := uuid.New()
	expected := []domain.Room{{Id: uuid.New(), Name: "Joined"}}
	memberSvc.On("GetRoomsByUserId", mock.Anything, memberId).Return(expected, nil).Once()

	c, rec := echotest.ContextConfig{
		PathValues: []echo.PathValue{{Name: "memberId", Value: memberId.String()}},
	}.ToContextRecorder(t)

	err := h.ListRoomsByMemberId(c)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)
	memberSvc.AssertExpectations(t)
}

func TestRoomHandler_ListUserRooms_Dedupes(t *testing.T) {
	svc := new(MockRoomService)
	memberSvc := new(MockRoomMemberService)
	h := NewRoomHandler(svc, memberSvc)

	userId := uuid.New()
	dupRoomId := uuid.New()
	owned := []domain.Room{{Id: dupRoomId, Name: "Owned"}, {Id: uuid.New(), Name: "Owned2"}}
	joined := []domain.Room{{Id: dupRoomId, Name: "Owned"}}

	svc.On("GetRoomsByOwnerId", mock.Anything, userId).Return(owned, nil).Once()
	memberSvc.On("GetRoomsByUserId", mock.Anything, userId).Return(joined, nil).Once()

	c, rec := echotest.ContextConfig{
		PathValues: []echo.PathValue{{Name: "id", Value: userId.String()}},
	}.ToContextRecorder(t)

	err := h.ListUserRooms(c)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)

	var resp response.Response[[]domain.Room]
	assert.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Len(t, resp.Data, 2)
	svc.AssertExpectations(t)
	memberSvc.AssertExpectations(t)
}

func TestRoomHandler_UpdateRoom_Success(t *testing.T) {
	svc := new(MockRoomService)
	h := NewRoomHandler(svc, new(MockRoomMemberService))

	roomId := uuid.New()
	expected := &domain.Room{Id: roomId, Name: "Renamed"}
	svc.On("UpdateRoom", mock.Anything, roomId, mock.MatchedBy(func(req structs.UpdateRoomRequest) bool {
		return req.Name != nil && *req.Name == "Renamed"
	})).Return(expected, nil).Once()

	c, rec := echotest.ContextConfig{
		PathValues: []echo.PathValue{{Name: "id", Value: roomId.String()}},
		Headers: map[string][]string{
			echo.HeaderContentType: {echo.MIMEApplicationJSON},
		},
		JSONBody: []byte(`{"name":"Renamed"}`),
	}.ToContextRecorder(t)

	err := h.UpdateRoom(c)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)
	svc.AssertExpectations(t)
}

func TestRoomHandler_UpdateRoom_InvalidID(t *testing.T) {
	svc := new(MockRoomService)
	h := NewRoomHandler(svc, new(MockRoomMemberService))

	c, rec := echotest.ContextConfig{
		PathValues: []echo.PathValue{{Name: "id", Value: "bad"}},
		Headers: map[string][]string{
			echo.HeaderContentType: {echo.MIMEApplicationJSON},
		},
		JSONBody: []byte(`{"name":"Renamed"}`),
	}.ToContextRecorder(t)

	err := h.UpdateRoom(c)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	svc.AssertNotCalled(t, "UpdateRoom", mock.Anything, mock.Anything, mock.Anything)
}

func TestRoomHandler_DeleteRoom_Success(t *testing.T) {
	svc := new(MockRoomService)
	h := NewRoomHandler(svc, new(MockRoomMemberService))

	roomId := uuid.New()
	svc.On("DeleteRoom", mock.Anything, roomId).Return(nil).Once()

	c, rec := echotest.ContextConfig{
		PathValues: []echo.PathValue{{Name: "id", Value: roomId.String()}},
	}.ToContextRecorder(t)

	err := h.DeleteRoom(c)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)
	svc.AssertExpectations(t)
}

func TestRoomHandler_DeleteRoom_Error(t *testing.T) {
	svc := new(MockRoomService)
	h := NewRoomHandler(svc, new(MockRoomMemberService))

	roomId := uuid.New()
	svc.On("DeleteRoom", mock.Anything, roomId).Return(errs.ErrRoomPopped).Once()

	c, rec := echotest.ContextConfig{
		PathValues: []echo.PathValue{{Name: "id", Value: roomId.String()}},
	}.ToContextRecorder(t)

	err := h.DeleteRoom(c)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusGone, rec.Code)
	svc.AssertExpectations(t)
}

func TestRoomHandler_PopRoom_Success(t *testing.T) {
	svc := new(MockRoomService)
	memberSvc := new(MockRoomMemberService)
	h := NewRoomHandler(svc, memberSvc)

	roomId := uuid.New()
	userId := uuid.New()

	memberSvc.On("HasManagerPermission", mock.Anything, userId, roomId).Return(true, nil).Once()
	svc.On("PopRoom", mock.Anything, roomId).Return(nil).Once()

	c, rec := echotest.ContextConfig{
		PathValues: []echo.PathValue{{Name: "id", Value: roomId.String()}},
	}.ToContextRecorder(t)
	setUserToken(c, userId)

	err := h.PopRoom(c)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)
	svc.AssertExpectations(t)
	memberSvc.AssertExpectations(t)
}

func TestRoomHandler_PopRoom_NotManager(t *testing.T) {
	svc := new(MockRoomService)
	memberSvc := new(MockRoomMemberService)
	h := NewRoomHandler(svc, memberSvc)

	roomId := uuid.New()
	userId := uuid.New()

	memberSvc.On("HasManagerPermission", mock.Anything, userId, roomId).Return(false, nil).Once()

	c, rec := echotest.ContextConfig{
		PathValues: []echo.PathValue{{Name: "id", Value: roomId.String()}},
	}.ToContextRecorder(t)
	setUserToken(c, userId)

	err := h.PopRoom(c)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusForbidden, rec.Code)
	svc.AssertNotCalled(t, "PopRoom", mock.Anything, mock.Anything)
	memberSvc.AssertExpectations(t)
}

func TestRoomHandler_PopRoom_Unauthorized(t *testing.T) {
	svc := new(MockRoomService)
	memberSvc := new(MockRoomMemberService)
	h := NewRoomHandler(svc, memberSvc)

	roomId := uuid.New()
	c, _ := echotest.ContextConfig{
		PathValues: []echo.PathValue{{Name: "id", Value: roomId.String()}},
	}.ToContextRecorder(t)

	err := h.PopRoom(c)
	assert.ErrorIs(t, err, echo.ErrUnauthorized)
	memberSvc.AssertNotCalled(t, "HasManagerPermission", mock.Anything, mock.Anything, mock.Anything)
}

func TestRoomHandler_GetRoom_DBConnError(t *testing.T) {
	svc := new(MockRoomService)
	h := NewRoomHandler(svc, new(MockRoomMemberService))

	roomId := uuid.New()
	// DB is down: the service propagates sql.ErrConnDone and the handler must
	// respond with a clean 500.
	svc.On("GetRoomById", mock.Anything, roomId).Return(nil, sql.ErrConnDone).Once()

	c, rec := echotest.ContextConfig{
		PathValues: []echo.PathValue{{Name: "id", Value: roomId.String()}},
	}.ToContextRecorder(t)

	err := h.GetRoom(c)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusInternalServerError, rec.Code)

	var resp response.Response[any]
	assert.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.False(t, resp.Success)
	assert.Equal(t, "Internal Server Error", resp.Message)
	assert.NotContains(t, rec.Body.String(), "sql:")
	svc.AssertExpectations(t)
}

func TestRoomHandler_DeleteRoom_DBConnError(t *testing.T) {
	svc := new(MockRoomService)
	h := NewRoomHandler(svc, new(MockRoomMemberService))

	roomId := uuid.New()
	svc.On("DeleteRoom", mock.Anything, roomId).Return(sql.ErrConnDone).Once()

	c, rec := echotest.ContextConfig{
		PathValues: []echo.PathValue{{Name: "id", Value: roomId.String()}},
	}.ToContextRecorder(t)

	err := h.DeleteRoom(c)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
	svc.AssertExpectations(t)
}
