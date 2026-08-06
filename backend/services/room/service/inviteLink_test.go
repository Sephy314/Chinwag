package service

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/Sephy314/chinwag/backend/services/room/domain"
	"github.com/Sephy314/chinwag/backend/services/room/shared/errs"
	"github.com/Sephy314/chinwag/backend/services/room/structs"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func newInviteLinkSvc(cache *MockCache, memberSvc *MockRoomMemberService, user *MockUserProvider, roomRepo *MockRoomRepo) *InviteLinkService {
	return NewInviteLinkService(cache, memberSvc, user, roomRepo)
}

func inviteDataJSON(roomId, createdBy string, singleUse bool) string {
	return fmt.Sprintf(`{"room_id":%q,"created_by":%q,"single_use":%v}`, roomId, createdBy, singleUse)
}

func TestInviteLinkService_CreateInviteLink_Success(t *testing.T) {
	cache := new(MockCache)
	memberSvc := new(MockRoomMemberService)
	user := new(MockUserProvider)
	roomRepo := new(MockRoomRepo)
	svc := newInviteLinkSvc(cache, memberSvc, user, roomRepo)

	roomId := uuid.New()
	createdBy := uuid.New()

	roomRepo.On("GetRoomById", mock.Anything, roomId).Return(domain.Room{Id: roomId}, nil).Once()
	memberSvc.On("HasManagerPermission", mock.Anything, createdBy, roomId).Return(true, nil).Once()
	cache.On("Set", mock.Anything, mock.MatchedBy(func(k string) bool {
		return len(k) > len(inviteKeyPrefix)
	}), mock.MatchedBy(func(v string) bool {
		return len(v) > 0
	}), defaultInviteTTL).Return(nil).Once()

	resp, err := svc.CreateInviteLink(context.Background(), roomId, createdBy, structs.CreateInviteLinkRequest{})

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, roomId.String(), resp.RoomId)
	assert.NotEmpty(t, resp.Token)
	assert.True(t, resp.ExpiresAt.After(time.Now()))
	cache.AssertExpectations(t)
	memberSvc.AssertExpectations(t)
	roomRepo.AssertExpectations(t)
}

func TestInviteLinkService_CreateInviteLink_CustomTTL(t *testing.T) {
	cache := new(MockCache)
	memberSvc := new(MockRoomMemberService)
	user := new(MockUserProvider)
	roomRepo := new(MockRoomRepo)
	svc := newInviteLinkSvc(cache, memberSvc, user, roomRepo)

	roomId := uuid.New()
	createdBy := uuid.New()
	ttlHours := 5

	roomRepo.On("GetRoomById", mock.Anything, roomId).Return(domain.Room{Id: roomId}, nil).Once()
	memberSvc.On("HasManagerPermission", mock.Anything, createdBy, roomId).Return(true, nil).Once()
	cache.On("Set", mock.Anything, mock.Anything, mock.Anything, 5*time.Hour).Return(nil).Once()

	resp, err := svc.CreateInviteLink(context.Background(), roomId, createdBy, structs.CreateInviteLinkRequest{TTLHours: &ttlHours})

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	cache.AssertExpectations(t)
}

func TestInviteLinkService_CreateInviteLink_SingleUse(t *testing.T) {
	cache := new(MockCache)
	memberSvc := new(MockRoomMemberService)
	user := new(MockUserProvider)
	roomRepo := new(MockRoomRepo)
	svc := newInviteLinkSvc(cache, memberSvc, user, roomRepo)

	roomId := uuid.New()
	createdBy := uuid.New()
	singleUse := true

	roomRepo.On("GetRoomById", mock.Anything, roomId).Return(domain.Room{Id: roomId}, nil).Once()
	memberSvc.On("HasManagerPermission", mock.Anything, createdBy, roomId).Return(true, nil).Once()
	cache.On("Set", mock.Anything, mock.Anything, mock.Anything, defaultInviteTTL).Return(nil).Once()

	resp, err := svc.CreateInviteLink(context.Background(), roomId, createdBy, structs.CreateInviteLinkRequest{SingleUse: &singleUse})

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	cache.AssertExpectations(t)
}

func TestInviteLinkService_CreateInviteLink_RoomNotFound(t *testing.T) {
	cache := new(MockCache)
	memberSvc := new(MockRoomMemberService)
	user := new(MockUserProvider)
	roomRepo := new(MockRoomRepo)
	svc := newInviteLinkSvc(cache, memberSvc, user, roomRepo)

	roomId := uuid.New()
	roomRepo.On("GetRoomById", mock.Anything, roomId).Return(domain.Room{}, errs.ErrNotFound).Once()

	resp, err := svc.CreateInviteLink(context.Background(), roomId, uuid.New(), structs.CreateInviteLinkRequest{})

	assert.Nil(t, resp)
	assert.ErrorIs(t, err, errs.ErrNotFound)
	memberSvc.AssertNotCalled(t, "HasManagerPermission", mock.Anything, mock.Anything, mock.Anything)
	roomRepo.AssertExpectations(t)
}

func TestInviteLinkService_CreateInviteLink_Popped(t *testing.T) {
	cache := new(MockCache)
	memberSvc := new(MockRoomMemberService)
	user := new(MockUserProvider)
	roomRepo := new(MockRoomRepo)
	svc := newInviteLinkSvc(cache, memberSvc, user, roomRepo)

	roomId := uuid.New()
	poppedAt := time.Now()
	roomRepo.On("GetRoomById", mock.Anything, roomId).Return(domain.Room{Id: roomId, PoppedAt: &poppedAt}, nil).Once()

	resp, err := svc.CreateInviteLink(context.Background(), roomId, uuid.New(), structs.CreateInviteLinkRequest{})

	assert.Nil(t, resp)
	assert.ErrorIs(t, err, errs.ErrRoomPopped)
	memberSvc.AssertNotCalled(t, "HasManagerPermission", mock.Anything, mock.Anything, mock.Anything)
	roomRepo.AssertExpectations(t)
}

func TestInviteLinkService_CreateInviteLink_NotAdmin(t *testing.T) {
	cache := new(MockCache)
	memberSvc := new(MockRoomMemberService)
	user := new(MockUserProvider)
	roomRepo := new(MockRoomRepo)
	svc := newInviteLinkSvc(cache, memberSvc, user, roomRepo)

	roomId := uuid.New()
	createdBy := uuid.New()

	roomRepo.On("GetRoomById", mock.Anything, roomId).Return(domain.Room{Id: roomId}, nil).Once()
	memberSvc.On("HasManagerPermission", mock.Anything, createdBy, roomId).Return(false, nil).Once()

	resp, err := svc.CreateInviteLink(context.Background(), roomId, createdBy, structs.CreateInviteLinkRequest{})

	assert.Nil(t, resp)
	assert.NotNil(t, err)
	var appErr *errs.AppError
	assert.ErrorAs(t, err, &appErr)
	assert.Equal(t, http.StatusForbidden, appErr.Status)
	cache.AssertNotCalled(t, "Set", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
	memberSvc.AssertExpectations(t)
}

func TestInviteLinkService_CreateInviteLink_HasManagerError(t *testing.T) {
	cache := new(MockCache)
	memberSvc := new(MockRoomMemberService)
	user := new(MockUserProvider)
	roomRepo := new(MockRoomRepo)
	svc := newInviteLinkSvc(cache, memberSvc, user, roomRepo)

	roomId := uuid.New()
	createdBy := uuid.New()

	roomRepo.On("GetRoomById", mock.Anything, roomId).Return(domain.Room{Id: roomId}, nil).Once()
	memberSvc.On("HasManagerPermission", mock.Anything, createdBy, roomId).Return(false, errors.New("boom")).Once()

	resp, err := svc.CreateInviteLink(context.Background(), roomId, createdBy, structs.CreateInviteLinkRequest{})

	assert.Nil(t, resp)
	assert.EqualError(t, err, "boom")
	cache.AssertNotCalled(t, "Set", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

func TestInviteLinkService_CreateInviteLink_CacheError(t *testing.T) {
	cache := new(MockCache)
	memberSvc := new(MockRoomMemberService)
	user := new(MockUserProvider)
	roomRepo := new(MockRoomRepo)
	svc := newInviteLinkSvc(cache, memberSvc, user, roomRepo)

	roomId := uuid.New()
	createdBy := uuid.New()

	roomRepo.On("GetRoomById", mock.Anything, roomId).Return(domain.Room{Id: roomId}, nil).Once()
	memberSvc.On("HasManagerPermission", mock.Anything, createdBy, roomId).Return(true, nil).Once()
	cache.On("Set", mock.Anything, mock.Anything, mock.Anything, defaultInviteTTL).Return(errors.New("redis down")).Once()

	resp, err := svc.CreateInviteLink(context.Background(), roomId, createdBy, structs.CreateInviteLinkRequest{})

	assert.Nil(t, resp)
	assert.EqualError(t, err, "redis down")
	cache.AssertExpectations(t)
}

func TestInviteLinkService_JoinByInviteLink_Success(t *testing.T) {
	cache := new(MockCache)
	memberSvc := new(MockRoomMemberService)
	user := new(MockUserProvider)
	roomRepo := new(MockRoomRepo)
	svc := newInviteLinkSvc(cache, memberSvc, user, roomRepo)

	roomId := uuid.New()
	userId := uuid.New()
	token := uuid.New().String()
	key := inviteKeyPrefix + token

	cache.On("Get", mock.Anything, key).Return(inviteDataJSON(roomId.String(), userId.String(), false), nil).Once()
	roomRepo.On("GetRoomById", mock.Anything, roomId).Return(domain.Room{Id: roomId}, nil).Once()
	user.On("GetUser", mock.Anything, userId.String()).Return(&UserInfo{Id: userId.String(), Name: "tester"}, nil).Once()
	memberSvc.On("GetUserByRoomIdAndUserId", mock.Anything, userId, roomId).Return(nil, errs.ErrNotFound).Once()
	memberSvc.On("InviteUser", mock.Anything, structs.RoomUser{UserId: userId, RoomId: roomId}).Return(nil).Once()

	got, err := svc.JoinByInviteLink(context.Background(), token, userId)

	assert.NoError(t, err)
	assert.Equal(t, roomId, got)
	cache.AssertExpectations(t)
	memberSvc.AssertExpectations(t)
	roomRepo.AssertExpectations(t)
	user.AssertExpectations(t)
}

func TestInviteLinkService_JoinByInviteLink_SingleUseDeletesCache(t *testing.T) {
	cache := new(MockCache)
	memberSvc := new(MockRoomMemberService)
	user := new(MockUserProvider)
	roomRepo := new(MockRoomRepo)
	svc := newInviteLinkSvc(cache, memberSvc, user, roomRepo)

	roomId := uuid.New()
	userId := uuid.New()
	token := uuid.New().String()
	key := inviteKeyPrefix + token

	cache.On("Get", mock.Anything, key).Return(inviteDataJSON(roomId.String(), userId.String(), true), nil).Once()
	roomRepo.On("GetRoomById", mock.Anything, roomId).Return(domain.Room{Id: roomId}, nil).Once()
	user.On("GetUser", mock.Anything, userId.String()).Return(&UserInfo{Id: userId.String()}, nil).Once()
	memberSvc.On("GetUserByRoomIdAndUserId", mock.Anything, userId, roomId).Return(nil, errs.ErrNotFound).Once()
	memberSvc.On("InviteUser", mock.Anything, structs.RoomUser{UserId: userId, RoomId: roomId}).Return(nil).Once()
	cache.On("Delete", mock.Anything, key).Return(nil).Once()

	got, err := svc.JoinByInviteLink(context.Background(), token, userId)

	assert.NoError(t, err)
	assert.Equal(t, roomId, got)
	cache.AssertExpectations(t)
	memberSvc.AssertExpectations(t)
}

func TestInviteLinkService_JoinByInviteLink_CacheMiss(t *testing.T) {
	cache := new(MockCache)
	memberSvc := new(MockRoomMemberService)
	user := new(MockUserProvider)
	roomRepo := new(MockRoomRepo)
	svc := newInviteLinkSvc(cache, memberSvc, user, roomRepo)

	token := uuid.New().String()
	cache.On("Get", mock.Anything, inviteKeyPrefix+token).Return("", errs.ErrCacheNotFound).Once()

	got, err := svc.JoinByInviteLink(context.Background(), token, uuid.New())

	assert.Equal(t, uuid.Nil, got)
	assert.ErrorIs(t, err, errs.ErrInviteNotFound)
	cache.AssertExpectations(t)
}

func TestInviteLinkService_JoinByInviteLink_InvalidData(t *testing.T) {
	cache := new(MockCache)
	memberSvc := new(MockRoomMemberService)
	user := new(MockUserProvider)
	roomRepo := new(MockRoomRepo)
	svc := newInviteLinkSvc(cache, memberSvc, user, roomRepo)

	token := uuid.New().String()
	cache.On("Get", mock.Anything, inviteKeyPrefix+token).Return("not-json{", nil).Once()

	got, err := svc.JoinByInviteLink(context.Background(), token, uuid.New())

	assert.Equal(t, uuid.Nil, got)
	assert.ErrorIs(t, err, errs.ErrInviteNotFound)
	cache.AssertExpectations(t)
}

func TestInviteLinkService_JoinByInviteLink_InvalidRoomIdInData(t *testing.T) {
	cache := new(MockCache)
	memberSvc := new(MockRoomMemberService)
	user := new(MockUserProvider)
	roomRepo := new(MockRoomRepo)
	svc := newInviteLinkSvc(cache, memberSvc, user, roomRepo)

	token := uuid.New().String()
	cache.On("Get", mock.Anything, inviteKeyPrefix+token).Return(`{"room_id":"not-a-uuid","created_by":"x","single_use":false}`, nil).Once()

	got, err := svc.JoinByInviteLink(context.Background(), token, uuid.New())

	assert.Equal(t, uuid.Nil, got)
	assert.ErrorIs(t, err, errs.ErrInviteNotFound)
	roomRepo.AssertNotCalled(t, "GetRoomById", mock.Anything, mock.Anything)
	cache.AssertExpectations(t)
}

func TestInviteLinkService_JoinByInviteLink_RoomNotFound(t *testing.T) {
	cache := new(MockCache)
	memberSvc := new(MockRoomMemberService)
	user := new(MockUserProvider)
	roomRepo := new(MockRoomRepo)
	svc := newInviteLinkSvc(cache, memberSvc, user, roomRepo)

	roomId := uuid.New()
	userId := uuid.New()
	token := uuid.New().String()
	key := inviteKeyPrefix + token

	cache.On("Get", mock.Anything, key).Return(inviteDataJSON(roomId.String(), userId.String(), false), nil).Once()
	roomRepo.On("GetRoomById", mock.Anything, roomId).Return(domain.Room{}, errs.ErrNotFound).Once()

	got, err := svc.JoinByInviteLink(context.Background(), token, userId)

	assert.Equal(t, uuid.Nil, got)
	assert.ErrorIs(t, err, errs.ErrNotFound)
	roomRepo.AssertExpectations(t)
	cache.AssertExpectations(t)
}

func TestInviteLinkService_JoinByInviteLink_RoomPopped(t *testing.T) {
	cache := new(MockCache)
	memberSvc := new(MockRoomMemberService)
	user := new(MockUserProvider)
	roomRepo := new(MockRoomRepo)
	svc := newInviteLinkSvc(cache, memberSvc, user, roomRepo)

	roomId := uuid.New()
	userId := uuid.New()
	token := uuid.New().String()
	key := inviteKeyPrefix + token
	poppedAt := time.Now()

	cache.On("Get", mock.Anything, key).Return(inviteDataJSON(roomId.String(), userId.String(), false), nil).Once()
	roomRepo.On("GetRoomById", mock.Anything, roomId).Return(domain.Room{Id: roomId, PoppedAt: &poppedAt}, nil).Once()

	got, err := svc.JoinByInviteLink(context.Background(), token, userId)

	assert.Equal(t, uuid.Nil, got)
	assert.ErrorIs(t, err, errs.ErrRoomPopped)
	roomRepo.AssertExpectations(t)
}

func TestInviteLinkService_JoinByInviteLink_UserDeleted(t *testing.T) {
	cache := new(MockCache)
	memberSvc := new(MockRoomMemberService)
	user := new(MockUserProvider)
	roomRepo := new(MockRoomRepo)
	svc := newInviteLinkSvc(cache, memberSvc, user, roomRepo)

	roomId := uuid.New()
	userId := uuid.New()
	token := uuid.New().String()
	key := inviteKeyPrefix + token

	cache.On("Get", mock.Anything, key).Return(inviteDataJSON(roomId.String(), userId.String(), false), nil).Once()
	roomRepo.On("GetRoomById", mock.Anything, roomId).Return(domain.Room{Id: roomId}, nil).Once()
	user.On("GetUser", mock.Anything, userId.String()).Return(nil, errs.ErrUserDeleted).Once()

	got, err := svc.JoinByInviteLink(context.Background(), token, userId)

	assert.Equal(t, uuid.Nil, got)
	assert.ErrorIs(t, err, errs.ErrUserDeleted)
	memberSvc.AssertNotCalled(t, "InviteUser", mock.Anything, mock.Anything)
	cache.AssertExpectations(t)
}

func TestInviteLinkService_JoinByInviteLink_AlreadyMember(t *testing.T) {
	cache := new(MockCache)
	memberSvc := new(MockRoomMemberService)
	user := new(MockUserProvider)
	roomRepo := new(MockRoomRepo)
	svc := newInviteLinkSvc(cache, memberSvc, user, roomRepo)

	roomId := uuid.New()
	userId := uuid.New()
	token := uuid.New().String()
	key := inviteKeyPrefix + token

	cache.On("Get", mock.Anything, key).Return(inviteDataJSON(roomId.String(), userId.String(), false), nil).Once()
	roomRepo.On("GetRoomById", mock.Anything, roomId).Return(domain.Room{Id: roomId}, nil).Once()
	user.On("GetUser", mock.Anything, userId.String()).Return(&UserInfo{Id: userId.String()}, nil).Once()
	memberSvc.On("GetUserByRoomIdAndUserId", mock.Anything, userId, roomId).Return(&domain.RoomMember{RoomId: roomId, UserId: userId}, nil).Once()

	got, err := svc.JoinByInviteLink(context.Background(), token, userId)

	assert.Equal(t, uuid.Nil, got)
	assert.ErrorIs(t, err, errs.ErrAlreadyMember)
	memberSvc.AssertNotCalled(t, "InviteUser", mock.Anything, mock.Anything)
	cache.AssertExpectations(t)
}

func TestInviteLinkService_JoinByInviteLink_InviteError(t *testing.T) {
	cache := new(MockCache)
	memberSvc := new(MockRoomMemberService)
	user := new(MockUserProvider)
	roomRepo := new(MockRoomRepo)
	svc := newInviteLinkSvc(cache, memberSvc, user, roomRepo)

	roomId := uuid.New()
	userId := uuid.New()
	token := uuid.New().String()
	key := inviteKeyPrefix + token

	cache.On("Get", mock.Anything, key).Return(inviteDataJSON(roomId.String(), userId.String(), false), nil).Once()
	roomRepo.On("GetRoomById", mock.Anything, roomId).Return(domain.Room{Id: roomId}, nil).Once()
	user.On("GetUser", mock.Anything, userId.String()).Return(&UserInfo{Id: userId.String()}, nil).Once()
	memberSvc.On("GetUserByRoomIdAndUserId", mock.Anything, userId, roomId).Return(nil, errs.ErrNotFound).Once()
	memberSvc.On("InviteUser", mock.Anything, structs.RoomUser{UserId: userId, RoomId: roomId}).Return(errors.New("cannot join")).Once()

	got, err := svc.JoinByInviteLink(context.Background(), token, userId)

	assert.Equal(t, uuid.Nil, got)
	assert.EqualError(t, err, "cannot join")
	memberSvc.AssertExpectations(t)
	cache.AssertExpectations(t)
}
