package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Sephy314/chinwag/room/domain"
	"github.com/Sephy314/chinwag/room/structs"
	"github.com/Sephy314/chinwag/shared/errs"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type MockRoomMemberRepo struct {
	mock.Mock
}

type MockRoomRepoForMember struct {
	mock.Mock
}

func (m *MockRoomRepoForMember) CreateRoom(ctx context.Context, room domain.Room) error {
	args := m.Called(ctx, room)
	return args.Error(0)
}

func (m *MockRoomRepoForMember) GetRoomById(ctx context.Context, roomId uuid.UUID) (domain.Room, error) {
	args := m.Called(ctx, roomId)
	return args.Get(0).(domain.Room), args.Error(1)
}

func (m *MockRoomRepoForMember) GetRoomsByOwnerId(ctx context.Context, ownerId uuid.UUID) ([]domain.Room, error) {
	args := m.Called(ctx, ownerId)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]domain.Room), args.Error(1)
}

func (m *MockRoomRepoForMember) UpdateRoom(ctx context.Context, room domain.Room) error {
	args := m.Called(ctx, room)
	return args.Error(0)
}

func (m *MockRoomRepoForMember) DeleteRoomById(ctx context.Context, roomId uuid.UUID) error {
	args := m.Called(ctx, roomId)
	return args.Error(0)
}

func newNotPoppedRoomRepo(roomId uuid.UUID) *MockRoomRepoForMember {
	mockRoomRepo := new(MockRoomRepoForMember)
	mockRoomRepo.On("GetRoomById", mock.Anything, roomId).Return(domain.Room{
		Id:    roomId,
		PopAt: time.Now().Add(24 * time.Hour),
	}, nil)
	return mockRoomRepo
}

func (m *MockRoomMemberRepo) GetMembersByRoomId(ctx context.Context, roomId uuid.UUID) ([]domain.RoomMember, error) {
	args := m.Called(ctx, roomId)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]domain.RoomMember), args.Error(1)
}

func (m *MockRoomMemberRepo) GetRoomsByUserId(ctx context.Context, userId uuid.UUID) ([]domain.Room, error) {
	args := m.Called(ctx, userId)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]domain.Room), args.Error(1)
}

func (m *MockRoomMemberRepo) GetMemberByRoomIdAndMemberId(ctx context.Context, roomId uuid.UUID, userId uuid.UUID) (domain.RoomMember, error) {
	args := m.Called(ctx, roomId, userId)
	return args.Get(0).(domain.RoomMember), args.Error(1)
}

func (m *MockRoomMemberRepo) AddMember(ctx context.Context, member domain.RoomMember) error {
	args := m.Called(ctx, member)
	return args.Error(0)
}

func (m *MockRoomMemberRepo) UpdateMember(ctx context.Context, member domain.RoomMember) error {
	args := m.Called(ctx, member)
	return args.Error(0)
}

func (m *MockRoomMemberRepo) RemoveMember(ctx context.Context, userId uuid.UUID, roomId uuid.UUID) error {
	args := m.Called(ctx, userId, roomId)
	return args.Error(0)
}

func (m *MockRoomMemberRepo) SetUserRole(ctx context.Context, userId uuid.UUID, roomId uuid.UUID, role domain.Role) error {
	args := m.Called(ctx, userId, roomId, role)
	return args.Error(0)
}

func TestInviteUser_Success(t *testing.T) {
	mockRepo := new(MockRoomMemberRepo)
	ctx := context.Background()

	userId := uuid.New()
	roomId := uuid.New()
	role := domain.MEMBER

	req := structs.RoomUser{
		UserId: userId,
		RoomId: roomId,
		Role:   &role,
	}

	mockRepo.On("AddMember", ctx, mock.MatchedBy(func(member domain.RoomMember) bool {
		return member.UserId == userId && member.RoomId == roomId && member.Role == role
	})).Return(nil)

	service := NewRoomMemberService(mockRepo, newNotPoppedRoomRepo(roomId), nil)
	err := service.InviteUser(ctx, req)

	assert.NoError(t, err)
	mockRepo.AssertExpectations(t)
}

func TestInviteUser_AlreadyExists(t *testing.T) {
	mockRepo := new(MockRoomMemberRepo)
	ctx := context.Background()

	userId := uuid.New()
	roomId := uuid.New()
	req := structs.RoomUser{
		UserId: userId,
		RoomId: roomId,
		Role:   new(domain.MEMBER),
	}

	mockRepo.On("AddMember", ctx, mock.MatchedBy(func(member domain.RoomMember) bool {
		return member.UserId == userId
	})).Return(errs.ErrConflict)

	service := NewRoomMemberService(mockRepo, newNotPoppedRoomRepo(roomId), nil)
	err := service.InviteUser(ctx, req)

	assert.Error(t, err)
	assert.Equal(t, errs.ErrConflict, err)
	mockRepo.AssertExpectations(t)
}

func TestInviteUser_Failed(t *testing.T) {
	mockRepo := new(MockRoomMemberRepo)
	ctx := context.Background()

	userId := uuid.New()
	roomId := uuid.New()
	req := structs.RoomUser{
		UserId: userId,
		RoomId: roomId,
		Role:   new(domain.MEMBER),
	}

	mockRepo.On("AddMember", ctx, mock.MatchedBy(func(member domain.RoomMember) bool {
		return member.UserId == userId
	})).Return(errors.New("database error"))

	service := NewRoomMemberService(mockRepo, newNotPoppedRoomRepo(roomId), nil)
	err := service.InviteUser(ctx, req)

	assert.Error(t, err)
	assert.Equal(t, "database error", err.Error())
	mockRepo.AssertExpectations(t)
}

func TestKickUser_Success(t *testing.T) {
	mockRepo := new(MockRoomMemberRepo)
	ctx := context.Background()

	userId := uuid.New()
	roomId := uuid.New()

	req := structs.RoomUser{
		UserId: userId,
		RoomId: roomId,
		Role:   nil,
	}

	mockRepo.On("RemoveMember", ctx, userId, roomId).Return(nil)

	service := NewRoomMemberService(mockRepo, newNotPoppedRoomRepo(roomId), nil)
	err := service.KickUser(ctx, req)

	assert.NoError(t, err)
	mockRepo.AssertExpectations(t)
}

func TestKickUser_NotFound(t *testing.T) {
	mockRepo := new(MockRoomMemberRepo)
	ctx := context.Background()

	userId := uuid.New()
	roomId := uuid.New()

	req := structs.RoomUser{
		UserId: userId,
		RoomId: roomId,
		Role:   nil,
	}

	mockRepo.On("RemoveMember", ctx, userId, roomId).Return(errs.ErrNotFound)

	service := NewRoomMemberService(mockRepo, newNotPoppedRoomRepo(roomId), nil)
	err := service.KickUser(ctx, req)

	assert.Error(t, err)
	assert.Equal(t, errs.ErrNotFound, err)
	mockRepo.AssertExpectations(t)
}

func TestGetUserByRoomId_Success(t *testing.T) {
	mockRepo := new(MockRoomMemberRepo)
	ctx := context.Background()

	roomId := uuid.New()
	userId1 := uuid.New()
	userId2 := uuid.New()

	members := []domain.RoomMember{
		{
			RoomId:   roomId,
			UserId:   userId1,
			Role:     domain.ADMIN,
			JoinedAt: time.Now(),
			LeftAt:   nil,
		},
		{
			RoomId:   roomId,
			UserId:   userId2,
			Role:     domain.MEMBER,
			JoinedAt: time.Now(),
			LeftAt:   nil,
		},
	}

	mockRepo.On("GetMembersByRoomId", ctx, roomId).Return(members, nil)

	service := NewRoomMemberService(mockRepo, new(MockRoomRepoForMember), nil)
	result, err := service.GetUserByRoomId(ctx, roomId)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, 2, len(result))
	assert.Equal(t, domain.ADMIN, result[0].Role)
	mockRepo.AssertExpectations(t)
}

func TestGetUserByRoomId_Empty(t *testing.T) {
	mockRepo := new(MockRoomMemberRepo)
	ctx := context.Background()

	roomId := uuid.New()
	var members []domain.RoomMember

	mockRepo.On("GetMembersByRoomId", ctx, roomId).Return(members, nil)

	service := NewRoomMemberService(mockRepo, new(MockRoomRepoForMember), nil)
	result, err := service.GetUserByRoomId(ctx, roomId)

	assert.NoError(t, err)
	assert.Equal(t, 0, len(result))
	mockRepo.AssertExpectations(t)
}

func TestGetUserByRoomId_Failed(t *testing.T) {
	mockRepo := new(MockRoomMemberRepo)
	ctx := context.Background()

	roomId := uuid.New()

	mockRepo.On("GetMembersByRoomId", ctx, roomId).Return(nil, errors.New("database error"))

	service := NewRoomMemberService(mockRepo, new(MockRoomRepoForMember), nil)
	result, err := service.GetUserByRoomId(ctx, roomId)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Equal(t, "database error", err.Error())
	mockRepo.AssertExpectations(t)
}

func TestGetRoomsByUserId_Success(t *testing.T) {
	mockRepo := new(MockRoomMemberRepo)
	ctx := context.Background()

	userId := uuid.New()
	roomId1 := uuid.New()
	roomId2 := uuid.New()

	rooms := []domain.Room{
		{
			Id:         roomId1,
			Name:       "Room 1",
			MaxMembers: 10,
			OwnerId:    uuid.New(),
		},
		{
			Id:         roomId2,
			Name:       "Room 2",
			MaxMembers: 20,
			OwnerId:    uuid.New(),
		},
	}

	mockRepo.On("GetRoomsByUserId", ctx, userId).Return(rooms, nil)

	service := NewRoomMemberService(mockRepo, new(MockRoomRepoForMember), nil)
	result, err := service.GetRoomsByUserId(ctx, userId)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, 2, len(result))
	assert.Equal(t, "Room 1", result[0].Name)
	mockRepo.AssertExpectations(t)
}

func TestGetRoomsByUserId_Empty(t *testing.T) {
	mockRepo := new(MockRoomMemberRepo)
	ctx := context.Background()

	userId := uuid.New()
	var rooms []domain.Room

	mockRepo.On("GetRoomsByUserId", ctx, userId).Return(rooms, nil)

	service := NewRoomMemberService(mockRepo, new(MockRoomRepoForMember), nil)
	result, err := service.GetRoomsByUserId(ctx, userId)

	assert.NoError(t, err)
	assert.Equal(t, 0, len(result))
	mockRepo.AssertExpectations(t)
}

func TestGetRoomsByUserId_Failed(t *testing.T) {
	mockRepo := new(MockRoomMemberRepo)
	ctx := context.Background()

	userId := uuid.New()

	mockRepo.On("GetRoomsByUserId", ctx, userId).Return(nil, errors.New("database error"))

	service := NewRoomMemberService(mockRepo, new(MockRoomRepoForMember), nil)
	result, err := service.GetRoomsByUserId(ctx, userId)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Equal(t, "database error", err.Error())
	mockRepo.AssertExpectations(t)
}

func TestSetUserRole_Success(t *testing.T) {
	mockRepo := new(MockRoomMemberRepo)
	ctx := context.Background()

	userId := uuid.New()
	roomId := uuid.New()
	role := domain.ADMIN

	mockRepo.On("SetUserRole", ctx, userId, roomId, role).Return(nil)

	service := NewRoomMemberService(mockRepo, newNotPoppedRoomRepo(roomId), nil)
	err := service.SetUserRole(ctx, userId, roomId, role)

	assert.NoError(t, err)
	mockRepo.AssertExpectations(t)
}

func TestSetUserRole_NotFound(t *testing.T) {
	mockRepo := new(MockRoomMemberRepo)
	ctx := context.Background()

	userId := uuid.New()
	roomId := uuid.New()
	role := domain.ADMIN

	mockRepo.On("SetUserRole", ctx, userId, roomId, role).Return(errs.ErrNotFound)

	service := NewRoomMemberService(mockRepo, newNotPoppedRoomRepo(roomId), nil)
	err := service.SetUserRole(ctx, userId, roomId, role)

	assert.Error(t, err)
	assert.Equal(t, errs.ErrNotFound, err)
	mockRepo.AssertExpectations(t)
}

func TestSetUserRole_Failed(t *testing.T) {
	mockRepo := new(MockRoomMemberRepo)
	ctx := context.Background()

	userId := uuid.New()
	roomId := uuid.New()
	role := domain.MEMBER

	mockRepo.On("SetUserRole", ctx, userId, roomId, role).Return(errors.New("database error"))

	service := NewRoomMemberService(mockRepo, newNotPoppedRoomRepo(roomId), nil)
	err := service.SetUserRole(ctx, userId, roomId, role)

	assert.Error(t, err)
	assert.Equal(t, "database error", err.Error())
	mockRepo.AssertExpectations(t)
}

func TestGetUserRole_Success(t *testing.T) {
	mockRepo := new(MockRoomMemberRepo)
	ctx := context.Background()

	userId := uuid.New()
	roomId := uuid.New()
	role := domain.ADMIN

	mockRepo.On("GetMemberByRoomIdAndMemberId", ctx, roomId, userId).Return(domain.RoomMember{
		UserId: userId,
		RoomId: roomId,
		Role:   role,
	}, nil)

	service := NewRoomMemberService(mockRepo, new(MockRoomRepoForMember), nil)
	result, err := service.GetUserRole(ctx, userId, roomId)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, role, *result)
	mockRepo.AssertExpectations(t)
}

func TestGetUserRole_NotFound(t *testing.T) {
	mockRepo := new(MockRoomMemberRepo)
	ctx := context.Background()

	userId := uuid.New()
	roomId := uuid.New()

	mockRepo.On("GetMemberByRoomIdAndMemberId", ctx, roomId, userId).Return(domain.RoomMember{}, errs.ErrNotFound)

	service := NewRoomMemberService(mockRepo, new(MockRoomRepoForMember), nil)
	result, err := service.GetUserRole(ctx, userId, roomId)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Equal(t, errs.ErrNotFound, err)
	mockRepo.AssertExpectations(t)
}

func TestIsManager_True(t *testing.T) {
	mockRepo := new(MockRoomMemberRepo)
	ctx := context.Background()

	userId := uuid.New()
	roomId := uuid.New()

	mockRepo.On("GetMemberByRoomIdAndMemberId", ctx, roomId, userId).Return(domain.RoomMember{
		UserId: userId,
		RoomId: roomId,
		Role:   domain.ADMIN,
	}, nil)

	service := NewRoomMemberService(mockRepo, new(MockRoomRepoForMember), nil)
	result, err := service.HasManagerPermission(ctx, userId, roomId)

	assert.NoError(t, err)
	assert.True(t, result)
	mockRepo.AssertExpectations(t)
}

func TestIsManager_False(t *testing.T) {
	mockRepo := new(MockRoomMemberRepo)
	ctx := context.Background()

	userId := uuid.New()
	roomId := uuid.New()

	mockRepo.On("GetMemberByRoomIdAndMemberId", ctx, roomId, userId).Return(domain.RoomMember{
		UserId: userId,
		RoomId: roomId,
		Role:   domain.MEMBER,
	}, nil)

	service := NewRoomMemberService(mockRepo, new(MockRoomRepoForMember), nil)
	result, err := service.HasManagerPermission(ctx, userId, roomId)

	assert.NoError(t, err)
	assert.False(t, result)
	mockRepo.AssertExpectations(t)
}

func TestIsManager_Error(t *testing.T) {
	mockRepo := new(MockRoomMemberRepo)
	ctx := context.Background()

	userId := uuid.New()
	roomId := uuid.New()

	mockRepo.On("GetMemberByRoomIdAndMemberId", ctx, roomId, userId).Return(domain.RoomMember{}, errs.ErrNotFound)

	service := NewRoomMemberService(mockRepo, new(MockRoomRepoForMember), nil)
	result, err := service.HasManagerPermission(ctx, userId, roomId)

	assert.Error(t, err)
	assert.False(t, result)
	assert.Equal(t, errs.ErrNotFound, err)
	mockRepo.AssertExpectations(t)
}

func TestUpdateRoomMember_Success(t *testing.T) {
	mockRepo := new(MockRoomMemberRepo)
	ctx := context.Background()

	userId := uuid.New()
	roomId := uuid.New()
	role := domain.ADMIN

	existingMember := domain.RoomMember{
		UserId: userId,
		RoomId: roomId,
		Role:   domain.MEMBER,
	}

	req := structs.UpdateRoomMemberRequest{
		Role: &role,
	}

	mockRepo.On("GetMemberByRoomIdAndMemberId", ctx, roomId, userId).Return(existingMember, nil)
	mockRepo.On("UpdateMember", ctx, mock.MatchedBy(func(member domain.RoomMember) bool {
		return member.UserId == userId && member.RoomId == roomId && member.Role == domain.ADMIN
	})).Return(nil)

	service := NewRoomMemberService(mockRepo, newNotPoppedRoomRepo(roomId), nil)
	result, err := service.UpdateRoomMember(ctx, userId, roomId, req)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, domain.ADMIN, result.Role)
	assert.Equal(t, userId, result.UserId)
	assert.Equal(t, roomId, result.RoomId)
	mockRepo.AssertExpectations(t)
}

func TestUpdateRoomMember_NotFound(t *testing.T) {
	mockRepo := new(MockRoomMemberRepo)
	ctx := context.Background()

	userId := uuid.New()
	roomId := uuid.New()

	mockRepo.On("GetMemberByRoomIdAndMemberId", ctx, roomId, userId).Return(domain.RoomMember{}, errs.ErrNotFound)

	service := NewRoomMemberService(mockRepo, newNotPoppedRoomRepo(roomId), nil)
	result, err := service.UpdateRoomMember(ctx, userId, roomId, structs.UpdateRoomMemberRequest{})

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Equal(t, errs.ErrNotFound, err)
	mockRepo.AssertExpectations(t)
}

func TestUpdateRoomMember_UpdateFailed(t *testing.T) {
	mockRepo := new(MockRoomMemberRepo)
	ctx := context.Background()

	userId := uuid.New()
	roomId := uuid.New()
	role := domain.ADMIN

	existingMember := domain.RoomMember{
		UserId: userId,
		RoomId: roomId,
		Role:   domain.MEMBER,
	}

	req := structs.UpdateRoomMemberRequest{
		Role: &role,
	}

	mockRepo.On("GetMemberByRoomIdAndMemberId", ctx, roomId, userId).Return(existingMember, nil)
	mockRepo.On("UpdateMember", ctx, mock.Anything).Return(errors.New("database error"))

	service := NewRoomMemberService(mockRepo, newNotPoppedRoomRepo(roomId), nil)
	result, err := service.UpdateRoomMember(ctx, userId, roomId, req)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Equal(t, "database error", err.Error())
	mockRepo.AssertExpectations(t)
}

func TestUpdateRoomMember_EmptyRequest(t *testing.T) {
	mockRepo := new(MockRoomMemberRepo)
	ctx := context.Background()

	userId := uuid.New()
	roomId := uuid.New()

	existingMember := domain.RoomMember{
		UserId: userId,
		RoomId: roomId,
		Role:   domain.MEMBER,
	}

	mockRepo.On("GetMemberByRoomIdAndMemberId", ctx, roomId, userId).Return(existingMember, nil)
	mockRepo.On("UpdateMember", ctx, mock.MatchedBy(func(member domain.RoomMember) bool {
		return member.Role == domain.MEMBER
	})).Return(nil)

	service := NewRoomMemberService(mockRepo, newNotPoppedRoomRepo(roomId), nil)
	result, err := service.UpdateRoomMember(ctx, userId, roomId, structs.UpdateRoomMemberRequest{})

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, domain.MEMBER, result.Role)
	mockRepo.AssertExpectations(t)
}

func newPoppedRoomRepo(roomId uuid.UUID) *MockRoomRepoForMember {
	mockRoomRepo := new(MockRoomRepoForMember)
	mockRoomRepo.On("GetRoomById", mock.Anything, roomId).Return(domain.Room{
		Id:       roomId,
		PoppedAt: &time.Time{},
		PopAt:    time.Now().Add(-24 * time.Hour),
	}, nil)
	return mockRoomRepo
}

func TestInviteUser_PoppedRoom(t *testing.T) {
	mockRepo := new(MockRoomMemberRepo)
	ctx := context.Background()

	roomId := uuid.New()
	req := structs.RoomUser{
		UserId: uuid.New(),
		RoomId: roomId,
	}

	service := NewRoomMemberService(mockRepo, newPoppedRoomRepo(roomId), nil)
	err := service.InviteUser(ctx, req)

	assert.Error(t, err)
	assert.Equal(t, errs.ErrRoomPopped, err)
	mockRepo.AssertExpectations(t)
}

func TestKickUser_PoppedRoom(t *testing.T) {
	mockRepo := new(MockRoomMemberRepo)
	ctx := context.Background()

	roomId := uuid.New()
	req := structs.RoomUser{
		UserId: uuid.New(),
		RoomId: roomId,
	}

	service := NewRoomMemberService(mockRepo, newPoppedRoomRepo(roomId), nil)
	err := service.KickUser(ctx, req)

	assert.Error(t, err)
	assert.Equal(t, errs.ErrRoomPopped, err)
	mockRepo.AssertExpectations(t)
}

func TestSetUserRole_PoppedRoom(t *testing.T) {
	mockRepo := new(MockRoomMemberRepo)
	ctx := context.Background()

	roomId := uuid.New()

	service := NewRoomMemberService(mockRepo, newPoppedRoomRepo(roomId), nil)
	err := service.SetUserRole(ctx, uuid.New(), roomId, domain.ADMIN)

	assert.Error(t, err)
	assert.Equal(t, errs.ErrRoomPopped, err)
	mockRepo.AssertExpectations(t)
}

func TestUpdateRoomMember_PoppedRoom(t *testing.T) {
	mockRepo := new(MockRoomMemberRepo)
	ctx := context.Background()

	roomId := uuid.New()

	service := NewRoomMemberService(mockRepo, newPoppedRoomRepo(roomId), nil)
	result, err := service.UpdateRoomMember(ctx, uuid.New(), roomId, structs.UpdateRoomMemberRequest{})

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Equal(t, errs.ErrRoomPopped, err)
	mockRepo.AssertExpectations(t)
}
