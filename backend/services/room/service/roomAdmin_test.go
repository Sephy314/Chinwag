package service

import (
	"context"
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

func TestRoomService_AdminCreateRoom_OwnerOverride(t *testing.T) {
	mockRepo := new(MockRoomRepo)
	mockMemberRepo := new(MockRoomMemberRepo)
	svc := NewRoomService(mockRepo, makeUow(mockRepo, mockMemberRepo))

	actor := uuid.New()
	owner := uuid.New()
	req := structs.AdminCreateRoomRequest{Name: "Admin Room", MaxMembers: 5, OwnerId: &owner}

	mockRepo.On("CreateRoom", mock.Anything, mock.MatchedBy(func(r domain.Room) bool {
		return r.Name == "Admin Room" && r.OwnerId == owner
	})).Return(nil).Once()
	mockMemberRepo.On("AddMember", mock.Anything, mock.MatchedBy(func(m domain.RoomMember) bool {
		return m.UserId == owner && m.Role == domain.ADMIN
	})).Return(nil).Once()

	room, err := svc.AdminCreateRoom(context.Background(), req, actor)
	assert.NoError(t, err)
	assert.NotNil(t, room)
	assert.Equal(t, owner, room.OwnerId)
	mockRepo.AssertExpectations(t)
	mockMemberRepo.AssertExpectations(t)
}

func TestRoomService_AdminCreateRoom_DefaultOwnerIsActor(t *testing.T) {
	mockRepo := new(MockRoomRepo)
	svc := NewRoomService(mockRepo)

	actor := uuid.New()
	req := structs.AdminCreateRoomRequest{Name: "Room"}

	mockRepo.On("CreateRoom", mock.Anything, mock.MatchedBy(func(r domain.Room) bool {
		return r.OwnerId == actor
	})).Return(nil).Once()

	room, err := svc.AdminCreateRoom(context.Background(), req, actor)
	assert.NoError(t, err)
	assert.Equal(t, actor, room.OwnerId)
	mockRepo.AssertExpectations(t)
}

func TestRoomService_AdminCreateRoom_PopAtInPast(t *testing.T) {
	mockRepo := new(MockRoomRepo)
	svc := NewRoomService(mockRepo)

	past := time.Now().Add(-time.Hour)
	req := structs.AdminCreateRoomRequest{Name: "Room", PopAt: &past}

	room, err := svc.AdminCreateRoom(context.Background(), req, uuid.New())
	assert.Nil(t, room)
	var appErr *errs.AppError
	assert.ErrorAs(t, err, &appErr)
	assert.Equal(t, http.StatusBadRequest, appErr.Status)
	mockRepo.AssertNotCalled(t, "CreateRoom", mock.Anything, mock.Anything)
}

func TestRoomService_AdminUpdateRoom_AllowsPopped(t *testing.T) {
	mockRepo := new(MockRoomRepo)
	svc := NewRoomService(mockRepo)

	id := uuid.New()
	existing := domain.Room{Id: id, Name: "old", MaxMembers: 10}
	mockRepo.On("GetRoomById", mock.Anything, id).Return(existing, nil).Once()
	mockRepo.On("AdminUpdateRoom", mock.Anything, mock.MatchedBy(func(r domain.Room) bool {
		return r.Name == "new" && r.MaxMembers == 20
	})).Return(nil).Once()

	name := "new"
	max := 20
	req := structs.AdminUpdateRoomRequest{Name: &name, MaxMembers: &max}
	room, err := svc.AdminUpdateRoom(context.Background(), id, req)
	assert.NoError(t, err)
	assert.Equal(t, "new", room.Name)
	mockRepo.AssertExpectations(t)
}

func TestRoomService_AdminUpdateRoom_GetFails(t *testing.T) {
	mockRepo := new(MockRoomRepo)
	svc := NewRoomService(mockRepo)

	id := uuid.New()
	mockRepo.On("GetRoomById", mock.Anything, id).Return(domain.Room{}, errs.ErrNotFound).Once()

	_, err := svc.AdminUpdateRoom(context.Background(), id, structs.AdminUpdateRoomRequest{})
	assert.ErrorIs(t, err, errs.ErrNotFound)
	mockRepo.AssertNotCalled(t, "AdminUpdateRoom", mock.Anything, mock.Anything)
}

func TestRoomService_AdminDeleteRoom_AllowsPopped(t *testing.T) {
	mockRepo := new(MockRoomRepo)
	svc := NewRoomService(mockRepo)

	id := uuid.New()
	mockRepo.On("AdminDeleteRoomById", mock.Anything, id).Return(nil).Once()

	err := svc.AdminDeleteRoom(context.Background(), id)
	assert.NoError(t, err)
	mockRepo.AssertExpectations(t)
}

func TestRoomService_AdminListRooms(t *testing.T) {
	mockRepo := new(MockRoomRepo)
	svc := NewRoomService(mockRepo)

	rooms := []domain.Room{{Id: uuid.New(), Name: "r1"}}
	mockRepo.On("ListRooms", mock.Anything, "cursor1", 25, "chat").Return(rooms, (*structs.CursorMeta)(nil), nil).Once()

	got, _, err := svc.AdminListRooms(context.Background(), structs.ListRoomsRequest{Cursor: "cursor1", Limit: 25, Search: "chat"})
	assert.NoError(t, err)
	assert.Len(t, got, 1)
	mockRepo.AssertExpectations(t)
}

func TestRoomService_AdminCountRooms(t *testing.T) {
	mockRepo := new(MockRoomRepo)
	svc := NewRoomService(mockRepo)

	mockRepo.On("CountRooms", mock.Anything).Return(42, nil).Once()

	n, err := svc.AdminCountRooms(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, int64(42), n)
	mockRepo.AssertExpectations(t)
}

func TestRoomMemberService_AdminInviteUser(t *testing.T) {
	mockRepo := new(MockRoomMemberRepo)
	svc := NewRoomMemberService(mockRepo, new(MockRoomRepo), new(MockUserProvider))

	roomId := uuid.New()
	userId := uuid.New()
	mockRepo.On("AdminAddMember", mock.Anything, mock.MatchedBy(func(m domain.RoomMember) bool {
		return m.RoomId == roomId && m.UserId == userId && m.Role == domain.MEMBER
	})).Return(nil).Once()

	err := svc.AdminInviteUser(context.Background(), structs.RoomUser{RoomId: roomId, UserId: userId})
	assert.NoError(t, err)
	mockRepo.AssertExpectations(t)
}

func TestRoomMemberService_AdminKickUser(t *testing.T) {
	mockRepo := new(MockRoomMemberRepo)
	svc := NewRoomMemberService(mockRepo, new(MockRoomRepo), new(MockUserProvider))

	roomId := uuid.New()
	userId := uuid.New()
	mockRepo.On("AdminRemoveMember", mock.Anything, userId, roomId).Return(nil).Once()

	err := svc.AdminKickUser(context.Background(), structs.RoomUser{RoomId: roomId, UserId: userId})
	assert.NoError(t, err)
	mockRepo.AssertExpectations(t)
}

func TestRoomMemberService_AdminUpdateRoomMember(t *testing.T) {
	mockRepo := new(MockRoomMemberRepo)
	svc := NewRoomMemberService(mockRepo, new(MockRoomRepo), new(MockUserProvider))

	roomId := uuid.New()
	userId := uuid.New()
	existing := domain.RoomMember{RoomId: roomId, UserId: userId, Role: domain.MEMBER}
	mockRepo.On("GetMemberByRoomIdAndMemberId", mock.Anything, roomId, userId).Return(existing, nil).Once()
	mockRepo.On("AdminUpdateMember", mock.Anything, mock.MatchedBy(func(m domain.RoomMember) bool {
		return m.Role == domain.ADMIN
	})).Return(nil).Once()

	role := domain.ADMIN
	member, err := svc.AdminUpdateRoomMember(context.Background(), userId, roomId, structs.UpdateRoomMemberRequest{Role: &role})
	assert.NoError(t, err)
	assert.Equal(t, domain.ADMIN, member.Role)
	mockRepo.AssertExpectations(t)
}
