package service

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/Sephy314/chinwag/conn/bridge"
	"github.com/Sephy314/chinwag/room/domain"
	"github.com/Sephy314/chinwag/room/structs"
	"github.com/Sephy314/chinwag/shared/errs"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type MockCache struct {
	mock.Mock
}

func (m *MockCache) Get(ctx context.Context, key string) (string, error) {
	args := m.Called(ctx, key)
	return args.String(0), args.Error(1)
}

func (m *MockCache) Set(ctx context.Context, key string, value any, ttl time.Duration) error {
	args := m.Called(ctx, key, value, ttl)
	return args.Error(0)
}

func (m *MockCache) Delete(ctx context.Context, key string) error {
	args := m.Called(ctx, key)
	return args.Error(0)
}

func (m *MockCache) TTL(ctx context.Context, key string) (time.Duration, error) {
	args := m.Called(ctx, key)
	return args.Get(0).(time.Duration), args.Error(1)
}

type MockUserProvider struct {
	mock.Mock
}

func (m *MockUserProvider) GetUser(ctx context.Context, id string) (*bridge.UserInfo, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*bridge.UserInfo), args.Error(1)
}

type MockRoomMemberService struct {
	mock.Mock
}

func (m *MockRoomMemberService) InviteUser(ctx context.Context, member structs.RoomUser) error {
	args := m.Called(ctx, member)
	return args.Error(0)
}

func (m *MockRoomMemberService) KickUser(ctx context.Context, member structs.RoomUser) error {
	args := m.Called(ctx, member)
	return args.Error(0)
}

func (m *MockRoomMemberService) GetUserByRoomId(ctx context.Context, roomId uuid.UUID) ([]domain.RoomMember, error) {
	args := m.Called(ctx, roomId)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]domain.RoomMember), args.Error(1)
}

func (m *MockRoomMemberService) GetRoomsByUserId(ctx context.Context, userId uuid.UUID) ([]domain.Room, error) {
	args := m.Called(ctx, userId)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]domain.Room), args.Error(1)
}

func (m *MockRoomMemberService) GetUserByRoomIdAndUserId(ctx context.Context, userId, roomId uuid.UUID) (*domain.RoomMember, error) {
	args := m.Called(ctx, userId, roomId)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.RoomMember), args.Error(1)
}

func (m *MockRoomMemberService) UpdateRoomMember(ctx context.Context, userId, roomId uuid.UUID, req structs.UpdateRoomMemberRequest) (*domain.RoomMember, error) {
	args := m.Called(ctx, userId, roomId, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.RoomMember), args.Error(1)
}

func (m *MockRoomMemberService) SetUserRole(ctx context.Context, userId uuid.UUID, roomId uuid.UUID, role domain.Role) error {
	args := m.Called(ctx, userId, roomId, role)
	return args.Error(0)
}

func (m *MockRoomMemberService) GetUserRole(ctx context.Context, userId uuid.UUID, roomId uuid.UUID) (*domain.Role, error) {
	args := m.Called(ctx, userId, roomId)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Role), args.Error(1)
}

func (m *MockRoomMemberService) HasManagerPermission(ctx context.Context, userId uuid.UUID, roomId uuid.UUID) (bool, error) {
	args := m.Called(ctx, userId, roomId)
	return args.Bool(0), args.Error(1)
}

func setupInviteLinkService(
	roomRepo *MockRoomRepoForMember,
	roomMemberSvc *MockRoomMemberService,
	userProvider *MockUserProvider,
	cache *MockCache,
) *InviteLinkService {
	return NewInviteLinkService(cache, roomMemberSvc, userProvider, roomRepo)
}

func TestCreateInviteLink_Success(t *testing.T) {
	ctx := context.Background()
	roomId := uuid.New()
	adminId := uuid.New()

	roomRepo := new(MockRoomRepoForMember)
	roomMemberSvc := new(MockRoomMemberService)
	userProvider := new(MockUserProvider)
	cache := new(MockCache)

	roomRepo.On("GetRoomById", ctx, roomId).Return(domain.Room{
		Id:    roomId,
		PopAt: time.Now().Add(24 * time.Hour),
	}, nil)

	roomMemberSvc.On("HasManagerPermission", ctx, adminId, roomId).Return(true, nil)

	cache.On("Set", ctx, mock.AnythingOfType("string"), mock.AnythingOfType("string"), 24*time.Hour).Return(nil)

	svc := setupInviteLinkService(roomRepo, roomMemberSvc, userProvider, cache)

	result, err := svc.CreateInviteLink(ctx, roomId, adminId, structs.CreateInviteLinkRequest{})

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, roomId.String(), result.RoomId)
	assert.NotEmpty(t, result.Token)
	assert.True(t, result.ExpiresAt.After(time.Now()))

	roomRepo.AssertExpectations(t)
	roomMemberSvc.AssertExpectations(t)
	cache.AssertExpectations(t)
}

func TestCreateInviteLink_NonAdmin(t *testing.T) {
	ctx := context.Background()
	roomId := uuid.New()
	userId := uuid.New()

	roomRepo := new(MockRoomRepoForMember)
	roomMemberSvc := new(MockRoomMemberService)
	userProvider := new(MockUserProvider)
	cache := new(MockCache)

	roomRepo.On("GetRoomById", ctx, roomId).Return(domain.Room{
		Id:    roomId,
		PopAt: time.Now().Add(24 * time.Hour),
	}, nil)

	roomMemberSvc.On("HasManagerPermission", ctx, userId, roomId).Return(false, nil)

	svc := setupInviteLinkService(roomRepo, roomMemberSvc, userProvider, cache)

	result, err := svc.CreateInviteLink(ctx, roomId, userId, structs.CreateInviteLinkRequest{})

	assert.Error(t, err)
	assert.Nil(t, result)

	var appErr *errs.AppError
	assert.True(t, errors.As(err, &appErr))
	assert.Equal(t, 403, appErr.Status)
	assert.Equal(t, "Admin permission is required", appErr.Message)

	roomRepo.AssertExpectations(t)
	roomMemberSvc.AssertExpectations(t)
}

func TestCreateInviteLink_UserNotInRoom(t *testing.T) {
	ctx := context.Background()
	roomId := uuid.New()
	userId := uuid.New()

	roomRepo := new(MockRoomRepoForMember)
	roomMemberSvc := new(MockRoomMemberService)
	userProvider := new(MockUserProvider)
	cache := new(MockCache)

	roomRepo.On("GetRoomById", ctx, roomId).Return(domain.Room{
		Id:    roomId,
		PopAt: time.Now().Add(24 * time.Hour),
	}, nil)

	roomMemberSvc.On("HasManagerPermission", ctx, userId, roomId).Return(false, errors.New("sql: no rows in result set"))

	svc := setupInviteLinkService(roomRepo, roomMemberSvc, userProvider, cache)

	result, err := svc.CreateInviteLink(ctx, roomId, userId, structs.CreateInviteLinkRequest{})

	assert.Error(t, err)
	assert.Nil(t, result)

	roomRepo.AssertExpectations(t)
	roomMemberSvc.AssertExpectations(t)
}

func TestCreateInviteLink_NonExistentRoom(t *testing.T) {
	ctx := context.Background()
	roomId := uuid.New()
	adminId := uuid.New()

	roomRepo := new(MockRoomRepoForMember)
	roomMemberSvc := new(MockRoomMemberService)
	userProvider := new(MockUserProvider)
	cache := new(MockCache)

	roomRepo.On("GetRoomById", ctx, roomId).Return(domain.Room{}, errors.New("sql: no rows in result set"))

	svc := setupInviteLinkService(roomRepo, roomMemberSvc, userProvider, cache)

	result, err := svc.CreateInviteLink(ctx, roomId, adminId, structs.CreateInviteLinkRequest{})

	assert.Error(t, err)
	assert.Nil(t, result)

	roomRepo.AssertExpectations(t)
}

func TestCreateInviteLink_PoppedRoom(t *testing.T) {
	ctx := context.Background()
	roomId := uuid.New()
	adminId := uuid.New()
	now := time.Now()

	roomRepo := new(MockRoomRepoForMember)
	roomMemberSvc := new(MockRoomMemberService)
	userProvider := new(MockUserProvider)
	cache := new(MockCache)

	roomRepo.On("GetRoomById", ctx, roomId).Return(domain.Room{
		Id:       roomId,
		PoppedAt: &now,
		PopAt:    time.Now().Add(-24 * time.Hour),
	}, nil)

	svc := setupInviteLinkService(roomRepo, roomMemberSvc, userProvider, cache)

	result, err := svc.CreateInviteLink(ctx, roomId, adminId, structs.CreateInviteLinkRequest{})

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Equal(t, errs.ErrRoomPopped, err)

	roomRepo.AssertExpectations(t)
}

func TestCreateInviteLink_CustomTTL(t *testing.T) {
	ctx := context.Background()
	roomId := uuid.New()
	adminId := uuid.New()
	ttlHours := 48

	roomRepo := new(MockRoomRepoForMember)
	roomMemberSvc := new(MockRoomMemberService)
	userProvider := new(MockUserProvider)
	cache := new(MockCache)

	roomRepo.On("GetRoomById", ctx, roomId).Return(domain.Room{
		Id:    roomId,
		PopAt: time.Now().Add(72 * time.Hour),
	}, nil)

	roomMemberSvc.On("HasManagerPermission", ctx, adminId, roomId).Return(true, nil)

	cache.On("Set", ctx, mock.AnythingOfType("string"), mock.AnythingOfType("string"), 48*time.Hour).Return(nil)

	svc := setupInviteLinkService(roomRepo, roomMemberSvc, userProvider, cache)

	result, err := svc.CreateInviteLink(ctx, roomId, adminId, structs.CreateInviteLinkRequest{
		TTLHours: &ttlHours,
	})

	assert.NoError(t, err)
	assert.NotNil(t, result)

	cache.AssertExpectations(t)
}

func TestCreateInviteLink_SingleUse(t *testing.T) {
	ctx := context.Background()
	roomId := uuid.New()
	adminId := uuid.New()
	singleUse := true

	roomRepo := new(MockRoomRepoForMember)
	roomMemberSvc := new(MockRoomMemberService)
	userProvider := new(MockUserProvider)
	cache := new(MockCache)

	roomRepo.On("GetRoomById", ctx, roomId).Return(domain.Room{
		Id:    roomId,
		PopAt: time.Now().Add(24 * time.Hour),
	}, nil)

	roomMemberSvc.On("HasManagerPermission", ctx, adminId, roomId).Return(true, nil)

	cache.On("Set", ctx, mock.AnythingOfType("string"), mock.AnythingOfType("string"), 24*time.Hour).Return(nil)

	svc := setupInviteLinkService(roomRepo, roomMemberSvc, userProvider, cache)

	result, err := svc.CreateInviteLink(ctx, roomId, adminId, structs.CreateInviteLinkRequest{
		SingleUse: &singleUse,
	})

	assert.NoError(t, err)
	assert.NotNil(t, result)

	roomRepo.AssertExpectations(t)
	roomMemberSvc.AssertExpectations(t)
	cache.AssertExpectations(t)
}

func TestJoinByInviteLink_Success(t *testing.T) {
	ctx := context.Background()
	roomId := uuid.New()
	userId := uuid.New()
	adminId := uuid.New()
	token := uuid.New().String()

	roomRepo := new(MockRoomRepoForMember)
	roomMemberSvc := new(MockRoomMemberService)
	userProvider := new(MockUserProvider)
	cache := new(MockCache)

	inviteData := inviteLinkData{
		RoomId:    roomId.String(),
		CreatedBy: adminId.String(),
		SingleUse: false,
	}
	jsonData, _ := json.Marshal(inviteData)

	cache.On("Get", ctx, inviteKeyPrefix+token).Return(string(jsonData), nil)

	roomRepo.On("GetRoomById", ctx, roomId).Return(domain.Room{
		Id:    roomId,
		PopAt: time.Now().Add(24 * time.Hour),
	}, nil)

	userProvider.On("GetUser", ctx, userId.String()).Return(&bridge.UserInfo{
		Id: userId.String(),
	}, nil)

	roomMemberSvc.On("GetUserByRoomIdAndUserId", ctx, userId, roomId).Return((*domain.RoomMember)(nil), errors.New("sql: no rows in result set"))

	roomMemberSvc.On("InviteUser", ctx, structs.RoomUser{
		UserId: userId,
		RoomId: roomId,
	}).Return(nil)

	svc := setupInviteLinkService(roomRepo, roomMemberSvc, userProvider, cache)

	err := svc.JoinByInviteLink(ctx, token, userId)

	assert.NoError(t, err)

	cache.AssertExpectations(t)
	roomRepo.AssertExpectations(t)
	userProvider.AssertExpectations(t)
	roomMemberSvc.AssertExpectations(t)
}

func TestJoinByInviteLink_AlreadyMember(t *testing.T) {
	ctx := context.Background()
	roomId := uuid.New()
	userId := uuid.New()
	adminId := uuid.New()
	token := uuid.New().String()

	roomRepo := new(MockRoomRepoForMember)
	roomMemberSvc := new(MockRoomMemberService)
	userProvider := new(MockUserProvider)
	cache := new(MockCache)

	inviteData := inviteLinkData{
		RoomId:    roomId.String(),
		CreatedBy: adminId.String(),
		SingleUse: false,
	}
	jsonData, _ := json.Marshal(inviteData)

	cache.On("Get", ctx, inviteKeyPrefix+token).Return(string(jsonData), nil)

	roomRepo.On("GetRoomById", ctx, roomId).Return(domain.Room{
		Id:    roomId,
		PopAt: time.Now().Add(24 * time.Hour),
	}, nil)

	userProvider.On("GetUser", ctx, userId.String()).Return(&bridge.UserInfo{
		Id: userId.String(),
	}, nil)

	roomMemberSvc.On("GetUserByRoomIdAndUserId", ctx, userId, roomId).Return(&domain.RoomMember{
		RoomId: roomId,
		UserId: userId,
	}, nil)

	svc := setupInviteLinkService(roomRepo, roomMemberSvc, userProvider, cache)

	err := svc.JoinByInviteLink(ctx, token, userId)

	assert.Error(t, err)
	assert.Equal(t, errs.ErrAlreadyMember, err)

	cache.AssertExpectations(t)
	roomMemberSvc.AssertExpectations(t)
}

func TestJoinByInviteLink_NonExistentUser(t *testing.T) {
	ctx := context.Background()
	roomId := uuid.New()
	userId := uuid.New()
	adminId := uuid.New()
	token := uuid.New().String()

	roomRepo := new(MockRoomRepoForMember)
	roomMemberSvc := new(MockRoomMemberService)
	userProvider := new(MockUserProvider)
	cache := new(MockCache)

	inviteData := inviteLinkData{
		RoomId:    roomId.String(),
		CreatedBy: adminId.String(),
		SingleUse: false,
	}
	jsonData, _ := json.Marshal(inviteData)

	cache.On("Get", ctx, inviteKeyPrefix+token).Return(string(jsonData), nil)

	roomRepo.On("GetRoomById", ctx, roomId).Return(domain.Room{
		Id:    roomId,
		PopAt: time.Now().Add(24 * time.Hour),
	}, nil)

	userProvider.On("GetUser", ctx, userId.String()).Return((*bridge.UserInfo)(nil), errors.New("sql: no rows in result set"))

	svc := setupInviteLinkService(roomRepo, roomMemberSvc, userProvider, cache)

	err := svc.JoinByInviteLink(ctx, token, userId)

	assert.Error(t, err)
	assert.Equal(t, errs.ErrUserDeleted, err)

	cache.AssertExpectations(t)
	userProvider.AssertExpectations(t)
}

func TestJoinByInviteLink_PoppedRoom(t *testing.T) {
	ctx := context.Background()
	roomId := uuid.New()
	userId := uuid.New()
	adminId := uuid.New()
	token := uuid.New().String()
	now := time.Now()

	roomRepo := new(MockRoomRepoForMember)
	roomMemberSvc := new(MockRoomMemberService)
	userProvider := new(MockUserProvider)
	cache := new(MockCache)

	inviteData := inviteLinkData{
		RoomId:    roomId.String(),
		CreatedBy: adminId.String(),
		SingleUse: false,
	}
	jsonData, _ := json.Marshal(inviteData)

	cache.On("Get", ctx, inviteKeyPrefix+token).Return(string(jsonData), nil)

	roomRepo.On("GetRoomById", ctx, roomId).Return(domain.Room{
		Id:       roomId,
		PoppedAt: &now,
		PopAt:    time.Now().Add(-24 * time.Hour),
	}, nil)

	svc := setupInviteLinkService(roomRepo, roomMemberSvc, userProvider, cache)

	err := svc.JoinByInviteLink(ctx, token, userId)

	assert.Error(t, err)
	assert.Equal(t, errs.ErrRoomPopped, err)

	cache.AssertExpectations(t)
	roomRepo.AssertExpectations(t)
}

func TestJoinByInviteLink_DeletedRoom(t *testing.T) {
	ctx := context.Background()
	roomId := uuid.New()
	userId := uuid.New()
	adminId := uuid.New()
	token := uuid.New().String()

	roomRepo := new(MockRoomRepoForMember)
	roomMemberSvc := new(MockRoomMemberService)
	userProvider := new(MockUserProvider)
	cache := new(MockCache)

	inviteData := inviteLinkData{
		RoomId:    roomId.String(),
		CreatedBy: adminId.String(),
		SingleUse: false,
	}
	jsonData, _ := json.Marshal(inviteData)

	cache.On("Get", ctx, inviteKeyPrefix+token).Return(string(jsonData), nil)

	roomRepo.On("GetRoomById", ctx, roomId).Return(domain.Room{}, errors.New("sql: no rows in result set"))

	svc := setupInviteLinkService(roomRepo, roomMemberSvc, userProvider, cache)

	err := svc.JoinByInviteLink(ctx, token, userId)

	assert.Error(t, err)

	cache.AssertExpectations(t)
	roomRepo.AssertExpectations(t)
}

func TestJoinByInviteLink_ExpiredToken(t *testing.T) {
	ctx := context.Background()
	userId := uuid.New()
	token := uuid.New().String()

	roomRepo := new(MockRoomRepoForMember)
	roomMemberSvc := new(MockRoomMemberService)
	userProvider := new(MockUserProvider)
	cache := new(MockCache)

	cache.On("Get", ctx, inviteKeyPrefix+token).Return("", errors.New("redis: nil"))

	svc := setupInviteLinkService(roomRepo, roomMemberSvc, userProvider, cache)

	err := svc.JoinByInviteLink(ctx, token, userId)

	assert.Error(t, err)
	assert.Equal(t, errs.ErrInviteNotFound, err)

	cache.AssertExpectations(t)
}

func TestJoinByInviteLink_SingleUseDeletesToken(t *testing.T) {
	ctx := context.Background()
	roomId := uuid.New()
	userId := uuid.New()
	adminId := uuid.New()
	token := uuid.New().String()

	roomRepo := new(MockRoomRepoForMember)
	roomMemberSvc := new(MockRoomMemberService)
	userProvider := new(MockUserProvider)
	cache := new(MockCache)

	inviteData := inviteLinkData{
		RoomId:    roomId.String(),
		CreatedBy: adminId.String(),
		SingleUse: true,
	}
	jsonData, _ := json.Marshal(inviteData)

	cache.On("Get", ctx, inviteKeyPrefix+token).Return(string(jsonData), nil)

	roomRepo.On("GetRoomById", ctx, roomId).Return(domain.Room{
		Id:    roomId,
		PopAt: time.Now().Add(24 * time.Hour),
	}, nil)

	userProvider.On("GetUser", ctx, userId.String()).Return(&bridge.UserInfo{
		Id: userId.String(),
	}, nil)

	roomMemberSvc.On("GetUserByRoomIdAndUserId", ctx, userId, roomId).Return((*domain.RoomMember)(nil), errors.New("sql: no rows in result set"))

	roomMemberSvc.On("InviteUser", ctx, structs.RoomUser{
		UserId: userId,
		RoomId: roomId,
	}).Return(nil)

	cache.On("Delete", ctx, inviteKeyPrefix+token).Return(nil)

	svc := setupInviteLinkService(roomRepo, roomMemberSvc, userProvider, cache)

	err := svc.JoinByInviteLink(ctx, token, userId)

	assert.NoError(t, err)

	cache.AssertCalled(t, "Delete", ctx, inviteKeyPrefix+token)
}

func TestJoinByInviteLink_MultiUseKeepsToken(t *testing.T) {
	ctx := context.Background()
	roomId := uuid.New()
	userId := uuid.New()
	adminId := uuid.New()
	token := uuid.New().String()

	roomRepo := new(MockRoomRepoForMember)
	roomMemberSvc := new(MockRoomMemberService)
	userProvider := new(MockUserProvider)
	cache := new(MockCache)

	inviteData := inviteLinkData{
		RoomId:    roomId.String(),
		CreatedBy: adminId.String(),
		SingleUse: false,
	}
	jsonData, _ := json.Marshal(inviteData)

	cache.On("Get", ctx, inviteKeyPrefix+token).Return(string(jsonData), nil)

	roomRepo.On("GetRoomById", ctx, roomId).Return(domain.Room{
		Id:    roomId,
		PopAt: time.Now().Add(24 * time.Hour),
	}, nil)

	userProvider.On("GetUser", ctx, userId.String()).Return(&bridge.UserInfo{
		Id: userId.String(),
	}, nil)

	roomMemberSvc.On("GetUserByRoomIdAndUserId", ctx, userId, roomId).Return((*domain.RoomMember)(nil), errors.New("sql: no rows in result set"))

	roomMemberSvc.On("InviteUser", ctx, structs.RoomUser{
		UserId: userId,
		RoomId: roomId,
	}).Return(nil)

	svc := setupInviteLinkService(roomRepo, roomMemberSvc, userProvider, cache)

	err := svc.JoinByInviteLink(ctx, token, userId)

	assert.NoError(t, err)

	cache.AssertNotCalled(t, "Delete", ctx, inviteKeyPrefix+token)
}

func TestJoinByInviteLink_UserServiceFailure(t *testing.T) {
	ctx := context.Background()
	roomId := uuid.New()
	userId := uuid.New()
	adminId := uuid.New()
	token := uuid.New().String()

	roomRepo := new(MockRoomRepoForMember)
	roomMemberSvc := new(MockRoomMemberService)
	userProvider := new(MockUserProvider)
	cache := new(MockCache)

	inviteData := inviteLinkData{
		RoomId:    roomId.String(),
		CreatedBy: adminId.String(),
		SingleUse: false,
	}
	jsonData, _ := json.Marshal(inviteData)

	cache.On("Get", ctx, inviteKeyPrefix+token).Return(string(jsonData), nil)

	roomRepo.On("GetRoomById", ctx, roomId).Return(domain.Room{
		Id:    roomId,
		PopAt: time.Now().Add(24 * time.Hour),
	}, nil)

	userProvider.On("GetUser", ctx, userId.String()).Return((*bridge.UserInfo)(nil), errors.New("user service unavailable"))

	svc := setupInviteLinkService(roomRepo, roomMemberSvc, userProvider, cache)

	err := svc.JoinByInviteLink(ctx, token, userId)

	assert.Error(t, err)
	assert.Equal(t, errs.ErrUserDeleted, err)

	cache.AssertExpectations(t)
	userProvider.AssertExpectations(t)
}

func TestJoinByInviteLink_InviteUserServiceDown(t *testing.T) {
	ctx := context.Background()
	roomId := uuid.New()
	userId := uuid.New()
	adminId := uuid.New()
	token := uuid.New().String()

	roomRepo := new(MockRoomRepoForMember)
	roomMemberSvc := new(MockRoomMemberService)
	userProvider := new(MockUserProvider)
	cache := new(MockCache)

	inviteData := inviteLinkData{
		RoomId:    roomId.String(),
		CreatedBy: adminId.String(),
		SingleUse: false,
	}
	jsonData, _ := json.Marshal(inviteData)

	cache.On("Get", ctx, inviteKeyPrefix+token).Return(string(jsonData), nil)

	roomRepo.On("GetRoomById", ctx, roomId).Return(domain.Room{
		Id:    roomId,
		PopAt: time.Now().Add(24 * time.Hour),
	}, nil)

	userProvider.On("GetUser", ctx, userId.String()).Return(&bridge.UserInfo{
		Id: userId.String(),
	}, nil)

	roomMemberSvc.On("GetUserByRoomIdAndUserId", ctx, userId, roomId).Return((*domain.RoomMember)(nil), errors.New("sql: no rows in result set"))

	roomMemberSvc.On("InviteUser", ctx, structs.RoomUser{
		UserId: userId,
		RoomId: roomId,
	}).Return(errors.New("database connection lost"))

	svc := setupInviteLinkService(roomRepo, roomMemberSvc, userProvider, cache)

	err := svc.JoinByInviteLink(ctx, token, userId)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "database connection lost")

	roomMemberSvc.AssertExpectations(t)
}
