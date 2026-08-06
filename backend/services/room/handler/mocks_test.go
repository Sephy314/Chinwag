package handler

import (
	"context"

	"github.com/Sephy314/chinwag/backend/services/room/domain"
	"github.com/Sephy314/chinwag/backend/services/room/service"
	"github.com/Sephy314/chinwag/backend/services/room/structs"
	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
)

type MockRoomService struct {
	mock.Mock
}

func (m *MockRoomService) CreateRoom(ctx context.Context, request structs.CreateRoomRequest) (*domain.Room, error) {
	args := m.Called(ctx, request)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Room), args.Error(1)
}

func (m *MockRoomService) GetRoomById(ctx context.Context, roomId uuid.UUID) (*domain.Room, error) {
	args := m.Called(ctx, roomId)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Room), args.Error(1)
}

func (m *MockRoomService) GetRoomsByOwnerId(ctx context.Context, ownerId uuid.UUID) ([]domain.Room, error) {
	args := m.Called(ctx, ownerId)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]domain.Room), args.Error(1)
}

func (m *MockRoomService) UpdateRoom(ctx context.Context, roomId uuid.UUID, req structs.UpdateRoomRequest) (*domain.Room, error) {
	args := m.Called(ctx, roomId, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Room), args.Error(1)
}

func (m *MockRoomService) DeleteRoom(ctx context.Context, roomId uuid.UUID) error {
	args := m.Called(ctx, roomId)
	return args.Error(0)
}

func (m *MockRoomService) PopRoom(ctx context.Context, roomId uuid.UUID) error {
	args := m.Called(ctx, roomId)
	return args.Error(0)
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

type MockUserProvider struct {
	mock.Mock
}

func (m *MockUserProvider) GetUser(ctx context.Context, id string) (*service.UserInfo, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*service.UserInfo), args.Error(1)
}

type MockInviteLinkService struct {
	mock.Mock
}

func (m *MockInviteLinkService) CreateInviteLink(ctx context.Context, roomId, createdBy uuid.UUID, req structs.CreateInviteLinkRequest) (*structs.InviteLinkResponse, error) {
	args := m.Called(ctx, roomId, createdBy, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*structs.InviteLinkResponse), args.Error(1)
}

func (m *MockInviteLinkService) JoinByInviteLink(ctx context.Context, token string, userId uuid.UUID) (uuid.UUID, error) {
	args := m.Called(ctx, token, userId)
	return args.Get(0).(uuid.UUID), args.Error(1)
}
