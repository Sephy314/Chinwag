package handler

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/Sephy314/chinwag/backend/services/room/domain"
	"github.com/Sephy314/chinwag/backend/services/room/shared/errs"
	"github.com/Sephy314/chinwag/backend/services/room/shared/response"
	"github.com/Sephy314/chinwag/backend/services/room/structs"
	sharedauth "github.com/Sephy314/chinwag/backend/shared/auth"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/labstack/echo/v5"
	"github.com/labstack/echo/v5/echotest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func setAdminRoomClaims(c *echo.Context) {
	claims := &sharedauth.Claims{Role: sharedauth.RoleAdmin, RegisteredClaims: jwt.RegisteredClaims{Subject: "11111111-1111-1111-1111-111111111111"}}
	c.Set(sharedauth.ClaimsContextKey, claims)
}

func TestAdminRoomHandler_ListRooms_Success(t *testing.T) {
	roomRepo := new(adminRoomRepoMock)
	h := newAdminRoomHandler(t, roomRepo, new(adminMemberRepoMock))

	rooms := []domain.Room{{Id: uuid.New(), Name: "General"}}
	roomRepo.On("ListRooms", mock.Anything, "", 0, "").Return(rooms, (*structs.CursorMeta)(nil), nil).Once()

	c, rec := echotest.ContextConfig{}.ToContextRecorder(t)
	setAdminRoomClaims(c)
	err := h.ListRooms(c)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)

	var resp response.Response[[]domain.Room]
	assert.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Len(t, resp.Data, 1)
	roomRepo.AssertExpectations(t)
}

func TestAdminRoomHandler_CreateRoom_Success(t *testing.T) {
	roomRepo := new(adminRoomRepoMock)
	h := newAdminRoomHandler(t, roomRepo, new(adminMemberRepoMock))

	owner := uuid.New()
	roomRepo.On("CreateRoom", mock.Anything, mock.MatchedBy(func(r domain.Room) bool {
		return r.Name == "Admin Room" && r.OwnerId == owner
	})).Return(nil).Once()

	c, rec := echotest.ContextConfig{
		Headers:  map[string][]string{echo.HeaderContentType: {echo.MIMEApplicationJSON}},
		JSONBody: []byte(`{"name":"Admin Room","max_members":5,"owner_id":"` + owner.String() + `"}`),
	}.ToContextRecorder(t)
	setAdminRoomClaims(c)

	err := h.CreateRoom(c)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusCreated, rec.Code)
	roomRepo.AssertExpectations(t)
}

func TestAdminRoomHandler_UpdateRoom_Success(t *testing.T) {
	roomRepo := new(adminRoomRepoMock)
	h := newAdminRoomHandler(t, roomRepo, new(adminMemberRepoMock))

	id := uuid.New()
	existing := domain.Room{Id: id, Name: "old", MaxMembers: 10}
	roomRepo.On("GetRoomById", mock.Anything, id).Return(existing, nil).Once()
	roomRepo.On("AdminUpdateRoom", mock.Anything, mock.MatchedBy(func(r domain.Room) bool {
		return r.Name == "new"
	})).Return(nil).Once()

	c, rec := echotest.ContextConfig{
		Headers:    map[string][]string{echo.HeaderContentType: {echo.MIMEApplicationJSON}},
		JSONBody:   []byte(`{"name":"new"}`),
		PathValues: []echo.PathValue{{Name: "id", Value: id.String()}},
	}.ToContextRecorder(t)
	setAdminRoomClaims(c)

	err := h.UpdateRoom(c)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)
	roomRepo.AssertExpectations(t)
}

func TestAdminRoomHandler_DeleteRoom_Success(t *testing.T) {
	roomRepo := new(adminRoomRepoMock)
	h := newAdminRoomHandler(t, roomRepo, new(adminMemberRepoMock))

	id := uuid.New()
	roomRepo.On("AdminDeleteRoomById", mock.Anything, id).Return(nil).Once()

	c, rec := echotest.ContextConfig{
		PathValues: []echo.PathValue{{Name: "id", Value: id.String()}},
	}.ToContextRecorder(t)
	setAdminRoomClaims(c)

	err := h.DeleteRoom(c)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)
	roomRepo.AssertExpectations(t)
}

func TestAdminRoomHandler_DeleteRoom_NotFound(t *testing.T) {
	roomRepo := new(adminRoomRepoMock)
	h := newAdminRoomHandler(t, roomRepo, new(adminMemberRepoMock))

	id := uuid.New()
	roomRepo.On("AdminDeleteRoomById", mock.Anything, id).Return(errs.ErrNotFound).Once()

	c, rec := echotest.ContextConfig{
		PathValues: []echo.PathValue{{Name: "id", Value: id.String()}},
	}.ToContextRecorder(t)
	setAdminRoomClaims(c)

	err := h.DeleteRoom(c)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusNotFound, rec.Code)
	roomRepo.AssertExpectations(t)
}

func TestAdminRoomHandler_ListMembers_Success(t *testing.T) {
	memberRepo := new(adminMemberRepoMock)
	h := newAdminRoomHandler(t, new(adminRoomRepoMock), memberRepo)

	roomId := uuid.New()
	members := []domain.RoomMember{{RoomId: roomId, UserId: uuid.New(), Role: domain.ADMIN, JoinedAt: time.Now()}}
	memberRepo.On("GetMembersByRoomId", mock.Anything, roomId).Return(members, nil).Once()

	c, rec := echotest.ContextConfig{
		PathValues: []echo.PathValue{{Name: "id", Value: roomId.String()}},
	}.ToContextRecorder(t)
	setAdminRoomClaims(c)

	err := h.ListMembers(c)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)
	memberRepo.AssertExpectations(t)
}

func TestAdminRoomHandler_AddMember_Success(t *testing.T) {
	memberRepo := new(adminMemberRepoMock)
	h := newAdminRoomHandler(t, new(adminRoomRepoMock), memberRepo)

	roomId := uuid.New()
	userId := uuid.New()
	memberRepo.On("AdminAddMember", mock.Anything, mock.MatchedBy(func(m domain.RoomMember) bool {
		return m.RoomId == roomId && m.UserId == userId
	})).Return(nil).Once()

	c, rec := echotest.ContextConfig{
		Headers:    map[string][]string{echo.HeaderContentType: {echo.MIMEApplicationJSON}},
		JSONBody:   []byte(`{"user_id":"` + userId.String() + `"}`),
		PathValues: []echo.PathValue{{Name: "id", Value: roomId.String()}},
	}.ToContextRecorder(t)
	setAdminRoomClaims(c)

	err := h.AddMember(c)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusCreated, rec.Code)
	memberRepo.AssertExpectations(t)
}

func TestAdminRoomHandler_RemoveMember_Success(t *testing.T) {
	memberRepo := new(adminMemberRepoMock)
	h := newAdminRoomHandler(t, new(adminRoomRepoMock), memberRepo)

	roomId := uuid.New()
	userId := uuid.New()
	memberRepo.On("AdminRemoveMember", mock.Anything, userId, roomId).Return(nil).Once()

	c, rec := echotest.ContextConfig{
		PathValues: []echo.PathValue{
			{Name: "id", Value: roomId.String()},
			{Name: "userId", Value: userId.String()},
		},
	}.ToContextRecorder(t)
	setAdminRoomClaims(c)

	err := h.RemoveMember(c)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)
	memberRepo.AssertExpectations(t)
}

func TestAdminRoomHandler_UpdateMember_Success(t *testing.T) {
	memberRepo := new(adminMemberRepoMock)
	h := newAdminRoomHandler(t, new(adminRoomRepoMock), memberRepo)

	roomId := uuid.New()
	userId := uuid.New()
	existing := domain.RoomMember{RoomId: roomId, UserId: userId, Role: domain.MEMBER}
	memberRepo.On("GetMemberByRoomIdAndMemberId", mock.Anything, roomId, userId).Return(existing, nil).Once()
	memberRepo.On("AdminUpdateMember", mock.Anything, mock.MatchedBy(func(m domain.RoomMember) bool {
		return m.Role == domain.ADMIN
	})).Return(nil).Once()

	c, rec := echotest.ContextConfig{
		Headers:  map[string][]string{echo.HeaderContentType: {echo.MIMEApplicationJSON}},
		JSONBody: []byte(`{"role":1}`),
		PathValues: []echo.PathValue{
			{Name: "id", Value: roomId.String()},
			{Name: "userId", Value: userId.String()},
		},
	}.ToContextRecorder(t)
	setAdminRoomClaims(c)

	err := h.UpdateMember(c)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)
	memberRepo.AssertExpectations(t)
}

func TestAdminRoomHandler_ListUserRooms_Success(t *testing.T) {
	memberRepo := new(adminMemberRepoMock)
	h := newAdminRoomHandler(t, new(adminRoomRepoMock), memberRepo)

	userId := uuid.New()
	rooms := []domain.Room{{Id: uuid.New(), Name: "r1"}}
	memberRepo.On("GetRoomsByUserId", mock.Anything, userId).Return(rooms, nil).Once()

	c, rec := echotest.ContextConfig{
		PathValues: []echo.PathValue{{Name: "userId", Value: userId.String()}},
	}.ToContextRecorder(t)
	setAdminRoomClaims(c)

	err := h.ListUserRooms(c)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)
	memberRepo.AssertExpectations(t)
}

func TestAdminRoomHandler_StatsRooms_Success(t *testing.T) {
	roomRepo := new(adminRoomRepoMock)
	h := newAdminRoomHandler(t, roomRepo, new(adminMemberRepoMock))

	roomRepo.On("CountRooms", mock.Anything).Return(int64(12), nil).Once()

	c, rec := echotest.ContextConfig{}.ToContextRecorder(t)
	setAdminRoomClaims(c)

	err := h.StatsRooms(c)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)

	var resp response.Response[map[string]int64]
	assert.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Equal(t, int64(12), resp.Data["total_rooms"])
	roomRepo.AssertExpectations(t)
}
