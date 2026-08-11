package handler

import (
	"context"
	"io"
	"log/slog"
	"testing"

	"github.com/Sephy314/chinwag/backend/services/room/domain"
	"github.com/Sephy314/chinwag/backend/services/room/repo"
	"github.com/Sephy314/chinwag/backend/services/room/service"
	"github.com/Sephy314/chinwag/backend/services/room/structs"
	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
)

// adminRoomRepoMock implements repo.RoomRepoInterface for handler tests.
type adminRoomRepoMock struct {
	mock.Mock
}

func (m *adminRoomRepoMock) CreateRoom(ctx context.Context, room domain.Room) error {
	args := m.Called(ctx, room)
	return args.Error(0)
}
func (m *adminRoomRepoMock) GetRoomById(ctx context.Context, id uuid.UUID) (domain.Room, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return domain.Room{}, args.Error(1)
	}
	return args.Get(0).(domain.Room), args.Error(1)
}
func (m *adminRoomRepoMock) GetRoomsByOwnerId(ctx context.Context, ownerId uuid.UUID) ([]domain.Room, error) {
	args := m.Called(ctx, ownerId)
	return args.Get(0).([]domain.Room), args.Error(1)
}
func (m *adminRoomRepoMock) UpdateRoom(ctx context.Context, room domain.Room) error {
	args := m.Called(ctx, room)
	return args.Error(0)
}
func (m *adminRoomRepoMock) DeleteRoomById(ctx context.Context, id uuid.UUID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}
func (m *adminRoomRepoMock) PopRoom(ctx context.Context, id uuid.UUID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}
func (m *adminRoomRepoMock) ListRooms(ctx context.Context, cursor string, limit int, search string) ([]domain.Room, *structs.CursorMeta, error) {
	args := m.Called(ctx, cursor, limit, search)
	var rooms []domain.Room
	if args.Get(0) != nil {
		rooms = args.Get(0).([]domain.Room)
	}
	var meta *structs.CursorMeta
	if args.Get(1) != nil {
		meta = args.Get(1).(*structs.CursorMeta)
	}
	return rooms, meta, args.Error(2)
}
func (m *adminRoomRepoMock) CountRooms(ctx context.Context) (int, error) {
	args := m.Called(ctx)
	return args.Int(0), args.Error(1)
}
func (m *adminRoomRepoMock) AdminUpdateRoom(ctx context.Context, room domain.Room) error {
	args := m.Called(ctx, room)
	return args.Error(0)
}
func (m *adminRoomRepoMock) AdminDeleteRoomById(ctx context.Context, id uuid.UUID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

// adminMemberRepoMock implements repo.RoomMemberRepoInterface for handler tests.
type adminMemberRepoMock struct {
	mock.Mock
}

func (m *adminMemberRepoMock) GetMembersByRoomId(ctx context.Context, roomId uuid.UUID) ([]domain.RoomMember, error) {
	args := m.Called(ctx, roomId)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]domain.RoomMember), args.Error(1)
}
func (m *adminMemberRepoMock) GetRoomsByUserId(ctx context.Context, userId uuid.UUID) ([]domain.Room, error) {
	args := m.Called(ctx, userId)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]domain.Room), args.Error(1)
}
func (m *adminMemberRepoMock) GetMemberByRoomIdAndMemberId(ctx context.Context, roomId, userId uuid.UUID) (domain.RoomMember, error) {
	args := m.Called(ctx, roomId, userId)
	if args.Get(0) == nil {
		return domain.RoomMember{}, args.Error(1)
	}
	return args.Get(0).(domain.RoomMember), args.Error(1)
}
func (m *adminMemberRepoMock) AddMember(ctx context.Context, member domain.RoomMember) error {
	args := m.Called(ctx, member)
	return args.Error(0)
}
func (m *adminMemberRepoMock) UpdateMember(ctx context.Context, member domain.RoomMember) error {
	args := m.Called(ctx, member)
	return args.Error(0)
}
func (m *adminMemberRepoMock) RemoveMember(ctx context.Context, userId, roomId uuid.UUID) error {
	args := m.Called(ctx, userId, roomId)
	return args.Error(0)
}
func (m *adminMemberRepoMock) SetUserRole(ctx context.Context, userId, roomId uuid.UUID, role domain.Role) error {
	args := m.Called(ctx, userId, roomId, role)
	return args.Error(0)
}
func (m *adminMemberRepoMock) AdminAddMember(ctx context.Context, member domain.RoomMember) error {
	args := m.Called(ctx, member)
	return args.Error(0)
}
func (m *adminMemberRepoMock) AdminUpdateMember(ctx context.Context, member domain.RoomMember) error {
	args := m.Called(ctx, member)
	return args.Error(0)
}
func (m *adminMemberRepoMock) AdminRemoveMember(ctx context.Context, userId, roomId uuid.UUID) error {
	args := m.Called(ctx, userId, roomId)
	return args.Error(0)
}
func (m *adminMemberRepoMock) AdminSetUserRole(ctx context.Context, userId, roomId uuid.UUID, role domain.Role) error {
	args := m.Called(ctx, userId, roomId, role)
	return args.Error(0)
}

// discardLogger returns a no-op slog logger for handler tests.
func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// newAdminRoomHandler builds the concrete services around mock repos and an
// audit client pointed at a no-op URL-less config.
func newAdminRoomHandler(t *testing.T, roomRepo repo.RoomRepoInterface, memberRepo repo.RoomMemberRepoInterface) *AdminRoomHandler {
	t.Helper()
	roomSvc := service.NewRoomService(roomRepo)
	memberSvc := service.NewRoomMemberService(memberRepo, roomRepo, nil)
	audit := service.NewAuditClient("", "", "", "", discardLogger())
	return NewAdminRoomHandler(roomSvc, memberSvc, audit, discardLogger())
}
