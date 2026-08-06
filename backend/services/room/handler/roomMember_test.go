package handler

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/Sephy314/chinwag/backend/services/room/domain"
	"github.com/Sephy314/chinwag/backend/services/room/service"
	"github.com/Sephy314/chinwag/backend/services/room/shared/errs"
	"github.com/Sephy314/chinwag/backend/services/room/shared/response"
	"github.com/Sephy314/chinwag/backend/services/room/structs"
	"github.com/google/uuid"
	"github.com/labstack/echo/v5"
	"github.com/labstack/echo/v5/echotest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func newMemberHandler(memberSvc *MockRoomMemberService, user service.UserProvider) *RoomMemberHandlerImpl {
	return NewRoomMemberHandler(memberSvc, new(MockRoomService), user)
}

func TestRoomMemberHandler_AddMember_Success(t *testing.T) {
	memberSvc := new(MockRoomMemberService)
	h := newMemberHandler(memberSvc, nil)

	roomId := uuid.New()
	managerId := uuid.New()
	newUserId := uuid.New()
	role := domain.ADMIN

	memberSvc.On("HasManagerPermission", mock.Anything, managerId, roomId).Return(true, nil).Once()
	memberSvc.On("InviteUser", mock.Anything, structs.RoomUser{UserId: newUserId, RoomId: roomId, Role: &role}).Return(nil).Once()

	c, rec := echotest.ContextConfig{
		PathValues: []echo.PathValue{{Name: "roomId", Value: roomId.String()}},
		Headers: map[string][]string{
			echo.HeaderContentType: {echo.MIMEApplicationJSON},
		},
		JSONBody: []byte(`{"user_id":"` + newUserId.String() + `","role":1}`),
	}.ToContextRecorder(t)
	setUserToken(c, managerId)

	err := h.AddMember(c)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusCreated, rec.Code)
	memberSvc.AssertExpectations(t)
}

func TestRoomMemberHandler_AddMember_NotManager(t *testing.T) {
	memberSvc := new(MockRoomMemberService)
	h := newMemberHandler(memberSvc, nil)

	roomId := uuid.New()
	managerId := uuid.New()
	newUserId := uuid.New()

	memberSvc.On("HasManagerPermission", mock.Anything, managerId, roomId).Return(false, nil).Once()

	c, rec := echotest.ContextConfig{
		PathValues: []echo.PathValue{{Name: "roomId", Value: roomId.String()}},
		Headers: map[string][]string{
			echo.HeaderContentType: {echo.MIMEApplicationJSON},
		},
		JSONBody: []byte(`{"user_id":"` + newUserId.String() + `"}`),
	}.ToContextRecorder(t)
	setUserToken(c, managerId)

	err := h.AddMember(c)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusForbidden, rec.Code)
	memberSvc.AssertNotCalled(t, "InviteUser", mock.Anything, mock.Anything)
	memberSvc.AssertExpectations(t)
}

func TestRoomMemberHandler_AddMember_InvalidRoomID(t *testing.T) {
	memberSvc := new(MockRoomMemberService)
	h := newMemberHandler(memberSvc, nil)

	c, rec := echotest.ContextConfig{
		PathValues: []echo.PathValue{{Name: "roomId", Value: "bad"}},
		Headers: map[string][]string{
			echo.HeaderContentType: {echo.MIMEApplicationJSON},
		},
		JSONBody: []byte(`{"user_id":"` + uuid.New().String() + `"}`),
	}.ToContextRecorder(t)
	setUserToken(c, uuid.New())

	err := h.AddMember(c)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	memberSvc.AssertNotCalled(t, "InviteUser", mock.Anything, mock.Anything)
}

func TestRoomMemberHandler_AddMember_Unauthorized(t *testing.T) {
	memberSvc := new(MockRoomMemberService)
	h := newMemberHandler(memberSvc, nil)

	c, rec := echotest.ContextConfig{
		PathValues: []echo.PathValue{{Name: "roomId", Value: uuid.New().String()}},
		Headers: map[string][]string{
			echo.HeaderContentType: {echo.MIMEApplicationJSON},
		},
		JSONBody: []byte(`{"user_id":"` + uuid.New().String() + `"}`),
	}.ToContextRecorder(t)

	// no user token set: IsManager fails and the handler maps the generic
	// error through errs.ParseError, which results in a 500 response.
	err := h.AddMember(c)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
	memberSvc.AssertNotCalled(t, "InviteUser", mock.Anything, mock.Anything)
}

func TestRoomMemberHandler_AddMember_InviteError(t *testing.T) {
	memberSvc := new(MockRoomMemberService)
	h := newMemberHandler(memberSvc, nil)

	roomId := uuid.New()
	managerId := uuid.New()
	newUserId := uuid.New()

	memberSvc.On("HasManagerPermission", mock.Anything, managerId, roomId).Return(true, nil).Once()
	memberSvc.On("InviteUser", mock.Anything, structs.RoomUser{UserId: newUserId, RoomId: roomId}).Return(errs.ErrConflict).Once()

	c, rec := echotest.ContextConfig{
		PathValues: []echo.PathValue{{Name: "roomId", Value: roomId.String()}},
		Headers: map[string][]string{
			echo.HeaderContentType: {echo.MIMEApplicationJSON},
		},
		JSONBody: []byte(`{"user_id":"` + newUserId.String() + `"}`),
	}.ToContextRecorder(t)
	setUserToken(c, managerId)

	err := h.AddMember(c)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusConflict, rec.Code)
	memberSvc.AssertExpectations(t)
}

func TestRoomMemberHandler_RemoveMember_Success(t *testing.T) {
	memberSvc := new(MockRoomMemberService)
	h := newMemberHandler(memberSvc, nil)

	roomId := uuid.New()
	managerId := uuid.New()
	kickId := uuid.New()

	memberSvc.On("HasManagerPermission", mock.Anything, managerId, roomId).Return(true, nil).Once()
	memberSvc.On("KickUser", mock.Anything, structs.RoomUser{UserId: kickId, RoomId: roomId}).Return(nil).Once()

	c, rec := echotest.ContextConfig{
		PathValues: []echo.PathValue{
			{Name: "roomId", Value: roomId.String()},
			{Name: "userId", Value: kickId.String()},
		},
	}.ToContextRecorder(t)
	setUserToken(c, managerId)

	err := h.RemoveMember(c)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)
	memberSvc.AssertExpectations(t)
}

func TestRoomMemberHandler_RemoveMember_InvalidUserID(t *testing.T) {
	memberSvc := new(MockRoomMemberService)
	h := newMemberHandler(memberSvc, nil)

	c, rec := echotest.ContextConfig{
		PathValues: []echo.PathValue{
			{Name: "roomId", Value: uuid.New().String()},
			{Name: "userId", Value: "bad"},
		},
	}.ToContextRecorder(t)
	setUserToken(c, uuid.New())

	err := h.RemoveMember(c)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	memberSvc.AssertNotCalled(t, "KickUser", mock.Anything, mock.Anything)
}

func TestRoomMemberHandler_RemoveMember_NotManager(t *testing.T) {
	memberSvc := new(MockRoomMemberService)
	h := newMemberHandler(memberSvc, nil)

	roomId := uuid.New()
	kickId := uuid.New()

	memberSvc.On("HasManagerPermission", mock.Anything, mock.Anything, roomId).Return(false, nil).Once()

	c, rec := echotest.ContextConfig{
		PathValues: []echo.PathValue{
			{Name: "roomId", Value: roomId.String()},
			{Name: "userId", Value: kickId.String()},
		},
	}.ToContextRecorder(t)
	setUserToken(c, uuid.New())

	err := h.RemoveMember(c)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusForbidden, rec.Code)
	memberSvc.AssertNotCalled(t, "KickUser", mock.Anything, mock.Anything)
	memberSvc.AssertExpectations(t)
}

func TestRoomMemberHandler_ListMembers_Success(t *testing.T) {
	memberSvc := new(MockRoomMemberService)
	user := new(MockUserProvider)
	h := newMemberHandler(memberSvc, user)

	roomId := uuid.New()
	memberA := uuid.New()
	memberB := uuid.New()
	joinedAt := time.Now()

	members := []domain.RoomMember{
		{RoomId: roomId, UserId: memberA, Role: domain.ADMIN, JoinedAt: joinedAt},
		{RoomId: roomId, UserId: memberB, Role: domain.MEMBER, JoinedAt: joinedAt},
	}
	memberSvc.On("GetUserByRoomId", mock.Anything, roomId).Return(members, nil).Once()
	user.On("GetUser", mock.Anything, memberA.String()).Return(&service.UserInfo{Id: memberA.String(), Name: "Alice"}, nil).Once()
	user.On("GetUser", mock.Anything, memberB.String()).Return(&service.UserInfo{Id: memberB.String(), Name: "Bob"}, nil).Once()

	c, rec := echotest.ContextConfig{
		PathValues: []echo.PathValue{{Name: "roomId", Value: roomId.String()}},
	}.ToContextRecorder(t)

	err := h.ListMembers(c)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)

	var resp response.Response[[]structs.RoomMemberResponse]
	assert.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Len(t, resp.Data, 2)
	assert.Equal(t, "Alice", resp.Data[0].UserName)
	assert.Equal(t, "Bob", resp.Data[1].UserName)
	memberSvc.AssertExpectations(t)
	user.AssertExpectations(t)
}

func TestRoomMemberHandler_ListMembers_Empty(t *testing.T) {
	memberSvc := new(MockRoomMemberService)
	h := newMemberHandler(memberSvc, nil)

	roomId := uuid.New()
	memberSvc.On("GetUserByRoomId", mock.Anything, roomId).Return([]domain.RoomMember{}, nil).Once()

	c, rec := echotest.ContextConfig{
		PathValues: []echo.PathValue{{Name: "roomId", Value: roomId.String()}},
	}.ToContextRecorder(t)

	err := h.ListMembers(c)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)

	var resp response.Response[[]structs.RoomMemberResponse]
	assert.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Empty(t, resp.Data)
	memberSvc.AssertExpectations(t)
}

func TestRoomMemberHandler_GetMember_Success(t *testing.T) {
	memberSvc := new(MockRoomMemberService)
	h := newMemberHandler(memberSvc, nil)

	roomId := uuid.New()
	userId := uuid.New()
	member := &domain.RoomMember{RoomId: roomId, UserId: userId, Role: domain.ADMIN, JoinedAt: time.Now()}

	memberSvc.On("GetUserByRoomIdAndUserId", mock.Anything, userId, roomId).Return(member, nil).Once()

	c, rec := echotest.ContextConfig{
		PathValues: []echo.PathValue{
			{Name: "roomId", Value: roomId.String()},
			{Name: "userId", Value: userId.String()},
		},
	}.ToContextRecorder(t)

	err := h.GetMember(c)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)
	memberSvc.AssertExpectations(t)
}

func TestRoomMemberHandler_GetMember_NotFound(t *testing.T) {
	memberSvc := new(MockRoomMemberService)
	h := newMemberHandler(memberSvc, nil)

	roomId := uuid.New()
	userId := uuid.New()
	memberSvc.On("GetUserByRoomIdAndUserId", mock.Anything, userId, roomId).Return(nil, errs.ErrNotFound).Once()

	c, rec := echotest.ContextConfig{
		PathValues: []echo.PathValue{
			{Name: "roomId", Value: roomId.String()},
			{Name: "userId", Value: userId.String()},
		},
	}.ToContextRecorder(t)

	err := h.GetMember(c)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusNotFound, rec.Code)
	memberSvc.AssertExpectations(t)
}

func TestRoomMemberHandler_UpdateMember_Success(t *testing.T) {
	memberSvc := new(MockRoomMemberService)
	h := newMemberHandler(memberSvc, nil)

	roomId := uuid.New()
	managerId := uuid.New()
	userId := uuid.New()
	role := domain.ADMIN
	expected := &domain.RoomMember{RoomId: roomId, UserId: userId, Role: domain.ADMIN}

	memberSvc.On("HasManagerPermission", mock.Anything, managerId, roomId).Return(true, nil).Once()
	memberSvc.On("UpdateRoomMember", mock.Anything, userId, roomId, structs.UpdateRoomMemberRequest{Role: &role}).Return(expected, nil).Once()

	c, rec := echotest.ContextConfig{
		PathValues: []echo.PathValue{
			{Name: "roomId", Value: roomId.String()},
			{Name: "userId", Value: userId.String()},
		},
		Headers: map[string][]string{
			echo.HeaderContentType: {echo.MIMEApplicationJSON},
		},
		JSONBody: []byte(`{"role":1}`),
	}.ToContextRecorder(t)
	setUserToken(c, managerId)

	err := h.UpdateMember(c)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)
	memberSvc.AssertExpectations(t)
}

func TestRoomMemberHandler_UpdateMember_NotManager(t *testing.T) {
	memberSvc := new(MockRoomMemberService)
	h := newMemberHandler(memberSvc, nil)

	roomId := uuid.New()
	userId := uuid.New()

	memberSvc.On("HasManagerPermission", mock.Anything, mock.Anything, roomId).Return(false, nil).Once()

	c, rec := echotest.ContextConfig{
		PathValues: []echo.PathValue{
			{Name: "roomId", Value: roomId.String()},
			{Name: "userId", Value: userId.String()},
		},
		Headers: map[string][]string{
			echo.HeaderContentType: {echo.MIMEApplicationJSON},
		},
		JSONBody: []byte(`{"role":1}`),
	}.ToContextRecorder(t)
	setUserToken(c, uuid.New())

	err := h.UpdateMember(c)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusForbidden, rec.Code)
	memberSvc.AssertNotCalled(t, "UpdateRoomMember", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
	memberSvc.AssertExpectations(t)
}

func TestRoomMemberHandler_UpdateMember_InvalidBody(t *testing.T) {
	memberSvc := new(MockRoomMemberService)
	h := newMemberHandler(memberSvc, nil)

	roomId := uuid.New()
	userId := uuid.New()

	memberSvc.On("HasManagerPermission", mock.Anything, mock.Anything, roomId).Return(true, nil).Once()

	c, rec := echotest.ContextConfig{
		PathValues: []echo.PathValue{
			{Name: "roomId", Value: roomId.String()},
			{Name: "userId", Value: userId.String()},
		},
		Headers: map[string][]string{
			echo.HeaderContentType: {echo.MIMEApplicationJSON},
		},
		JSONBody: []byte(`{bad`),
	}.ToContextRecorder(t)
	setUserToken(c, uuid.New())

	err := h.UpdateMember(c)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	memberSvc.AssertNotCalled(t, "UpdateRoomMember", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
	memberSvc.AssertExpectations(t)
}
