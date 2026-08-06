package service

import (
	"context"
	"time"

	"github.com/Sephy314/chinwag/backend/services/room/domain"
	"github.com/Sephy314/chinwag/backend/services/room/repo"
	"github.com/Sephy314/chinwag/backend/services/room/structs"
	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
)

// ---- Repos ----

type MockRoomRepo struct {
	mock.Mock
}

func (m *MockRoomRepo) CreateRoom(ctx context.Context, room domain.Room) error {
	args := m.Called(ctx, room)
	return args.Error(0)
}

func (m *MockRoomRepo) GetRoomById(ctx context.Context, id uuid.UUID) (domain.Room, error) {
	args := m.Called(ctx, id)
	return args.Get(0).(domain.Room), args.Error(1)
}

func (m *MockRoomRepo) GetRoomsByOwnerId(ctx context.Context, ownerId uuid.UUID) ([]domain.Room, error) {
	args := m.Called(ctx, ownerId)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]domain.Room), args.Error(1)
}

func (m *MockRoomRepo) UpdateRoom(ctx context.Context, room domain.Room) error {
	args := m.Called(ctx, room)
	return args.Error(0)
}

func (m *MockRoomRepo) DeleteRoomById(ctx context.Context, id uuid.UUID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockRoomRepo) PopRoom(ctx context.Context, id uuid.UUID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

type MockRoomMemberRepo struct {
	mock.Mock
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

func (m *MockRoomMemberRepo) GetMemberByRoomIdAndMemberId(ctx context.Context, roomId, userId uuid.UUID) (domain.RoomMember, error) {
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

func (m *MockRoomMemberRepo) RemoveMember(ctx context.Context, userId, roomId uuid.UUID) error {
	args := m.Called(ctx, userId, roomId)
	return args.Error(0)
}

func (m *MockRoomMemberRepo) SetUserRole(ctx context.Context, userId, roomId uuid.UUID, role domain.Role) error {
	args := m.Called(ctx, userId, roomId, role)
	return args.Error(0)
}

// ---- Unit of work ----

type testTransaction struct {
	roomRepo       repo.RoomRepoInterface
	roomMemberRepo repo.RoomMemberRepoInterface
}

func (t *testTransaction) RoomRepo() repo.RoomRepoInterface {
	return t.roomRepo
}

func (t *testTransaction) RoomMemberRepo() repo.RoomMemberRepoInterface {
	return t.roomMemberRepo
}

type testUnitOfWork struct {
	roomRepo       repo.RoomRepoInterface
	roomMemberRepo repo.RoomMemberRepoInterface
}

func (u *testUnitOfWork) Do(ctx context.Context, fn func(ctx context.Context, tx repo.Transaction) error) error {
	return fn(ctx, &testTransaction{roomRepo: u.roomRepo, roomMemberRepo: u.roomMemberRepo})
}

func makeUow(roomRepo repo.RoomRepoInterface, memberRepo repo.RoomMemberRepoInterface) repo.UnitOfWork {
	return &testUnitOfWork{roomRepo: roomRepo, roomMemberRepo: memberRepo}
}

// ---- Cache ----

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

// ---- User provider ----

type MockUserProvider struct {
	mock.Mock
}

func (m *MockUserProvider) GetUser(ctx context.Context, id string) (*UserInfo, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*UserInfo), args.Error(1)
}

// ---- Room member service (for invite link tests) ----

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

func (m *MockRoomMemberService) GetRoomById(ctx context.Context, roomId uuid.UUID) (domain.Room, error) {
	args := m.Called(ctx, roomId)
	return args.Get(0).(domain.Room), args.Error(1)
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

func (m *MockRoomMemberService) SetUserRole(ctx context.Context, userId, roomId uuid.UUID, role domain.Role) error {
	args := m.Called(ctx, userId, roomId, role)
	return args.Error(0)
}

func (m *MockRoomMemberService) GetUserRole(ctx context.Context, userId, roomId uuid.UUID) (*domain.Role, error) {
	args := m.Called(ctx, userId, roomId)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Role), args.Error(1)
}

func (m *MockRoomMemberService) HasManagerPermission(ctx context.Context, userId, roomId uuid.UUID) (bool, error) {
	args := m.Called(ctx, userId, roomId)
	return args.Bool(0), args.Error(1)
}
