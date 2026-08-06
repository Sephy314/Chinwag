package service

import (
	"context"
	"errors"
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

func TestRoomService_CreateRoom_Success_NoUow(t *testing.T) {
	mockRepo := new(MockRoomRepo)
	svc := NewRoomService(mockRepo)

	ownerId := uuid.New()
	ctx := context.WithValue(context.Background(), "ownerId", ownerId)
	req := structs.CreateRoomRequest{Name: "General", MaxMembers: 10}

	mockRepo.On("CreateRoom", mock.Anything, mock.MatchedBy(func(r domain.Room) bool {
		return r.Name == "General" && r.MaxMembers == 10 && r.OwnerId == ownerId && !r.PopAt.IsZero()
	})).Return(nil).Once()

	room, err := svc.CreateRoom(ctx, req)

	assert.NoError(t, err)
	assert.NotNil(t, room)
	assert.Equal(t, "General", room.Name)
	assert.Equal(t, ownerId, room.OwnerId)
	assert.True(t, room.PopAt.After(time.Now().Add(-time.Minute)))
	mockRepo.AssertExpectations(t)
}

func TestRoomService_CreateRoom_Success_WithUow(t *testing.T) {
	mockRepo := new(MockRoomRepo)
	mockMemberRepo := new(MockRoomMemberRepo)
	svc := NewRoomService(mockRepo, makeUow(mockRepo, mockMemberRepo))

	ownerId := uuid.New()
	ctx := context.WithValue(context.Background(), "ownerId", ownerId)
	req := structs.CreateRoomRequest{Name: "General", MaxMembers: 10}

	mockRepo.On("CreateRoom", mock.Anything, mock.MatchedBy(func(r domain.Room) bool {
		return r.Name == "General" && r.OwnerId == ownerId
	})).Return(nil).Once()
	mockMemberRepo.On("AddMember", mock.Anything, mock.MatchedBy(func(m domain.RoomMember) bool {
		return m.UserId == ownerId && m.Role == domain.ADMIN
	})).Return(nil).Once()

	room, err := svc.CreateRoom(ctx, req)

	assert.NoError(t, err)
	assert.NotNil(t, room)
	mockRepo.AssertExpectations(t)
	mockMemberRepo.AssertExpectations(t)
}

func TestRoomService_CreateRoom_Unauthorized(t *testing.T) {
	mockRepo := new(MockRoomRepo)
	svc := NewRoomService(mockRepo)

	room, err := svc.CreateRoom(context.Background(), structs.CreateRoomRequest{Name: "General"})

	assert.Nil(t, room)
	assert.NotNil(t, err)
	var appErr *errs.AppError
	assert.ErrorAs(t, err, &appErr)
	assert.Equal(t, http.StatusUnauthorized, appErr.Status)
	mockRepo.AssertNotCalled(t, "CreateRoom", mock.Anything, mock.Anything)
}

func TestRoomService_CreateRoom_PopAtInPast(t *testing.T) {
	mockRepo := new(MockRoomRepo)
	svc := NewRoomService(mockRepo)

	ownerId := uuid.New()
	ctx := context.WithValue(context.Background(), "ownerId", ownerId)
	past := time.Now().Add(-time.Hour)
	req := structs.CreateRoomRequest{Name: "General", PopAt: &past}

	room, err := svc.CreateRoom(ctx, req)

	assert.Nil(t, room)
	assert.NotNil(t, err)
	var appErr *errs.AppError
	assert.ErrorAs(t, err, &appErr)
	assert.Equal(t, http.StatusBadRequest, appErr.Status)
	mockRepo.AssertNotCalled(t, "CreateRoom", mock.Anything, mock.Anything)
}

func TestRoomService_CreateRoom_PopAtInFuture(t *testing.T) {
	mockRepo := new(MockRoomRepo)
	svc := NewRoomService(mockRepo)

	ownerId := uuid.New()
	ctx := context.WithValue(context.Background(), "ownerId", ownerId)
	future := time.Now().Add(2 * time.Hour)
	req := structs.CreateRoomRequest{Name: "General", PopAt: &future}

	mockRepo.On("CreateRoom", mock.Anything, mock.MatchedBy(func(r domain.Room) bool {
		return r.PopAt.Equal(future)
	})).Return(nil).Once()

	room, err := svc.CreateRoom(ctx, req)

	assert.NoError(t, err)
	assert.NotNil(t, room)
	assert.True(t, room.PopAt.Equal(future))
	mockRepo.AssertExpectations(t)
}

func TestRoomService_CreateRoom_RepoError(t *testing.T) {
	mockRepo := new(MockRoomRepo)
	svc := NewRoomService(mockRepo)

	ownerId := uuid.New()
	ctx := context.WithValue(context.Background(), "ownerId", ownerId)
	req := structs.CreateRoomRequest{Name: "General"}

	mockRepo.On("CreateRoom", mock.Anything, mock.Anything).Return(errors.New("db down")).Once()

	room, err := svc.CreateRoom(ctx, req)

	assert.NotNil(t, room) // service returns the room alongside the error
	assert.Error(t, err)
	assert.EqualError(t, err, "db down")
	mockRepo.AssertExpectations(t)
}

func TestRoomService_CreateRoom_UowError(t *testing.T) {
	mockRepo := new(MockRoomRepo)
	mockMemberRepo := new(MockRoomMemberRepo)
	svc := NewRoomService(mockRepo, makeUow(mockRepo, mockMemberRepo))

	ownerId := uuid.New()
	ctx := context.WithValue(context.Background(), "ownerId", ownerId)
	req := structs.CreateRoomRequest{Name: "General"}

	mockRepo.On("CreateRoom", mock.Anything, mock.Anything).Return(errors.New("tx failed")).Once()

	room, err := svc.CreateRoom(ctx, req)

	assert.NotNil(t, room)
	assert.Error(t, err)
	assert.EqualError(t, err, "tx failed")
	mockRepo.AssertExpectations(t)
}

func TestRoomService_GetRoomById_Success(t *testing.T) {
	mockRepo := new(MockRoomRepo)
	svc := NewRoomService(mockRepo)

	roomId := uuid.New()
	expected := domain.Room{Id: roomId, Name: "General", OwnerId: uuid.New()}

	mockRepo.On("GetRoomById", mock.Anything, roomId).Return(expected, nil).Once()

	room, err := svc.GetRoomById(context.Background(), roomId)

	assert.NoError(t, err)
	assert.NotNil(t, room)
	assert.Equal(t, "General", room.Name)
	mockRepo.AssertExpectations(t)
}

func TestRoomService_GetRoomById_Error(t *testing.T) {
	mockRepo := new(MockRoomRepo)
	svc := NewRoomService(mockRepo)

	roomId := uuid.New()
	mockRepo.On("GetRoomById", mock.Anything, roomId).Return(domain.Room{}, errs.ErrNotFound).Once()

	room, err := svc.GetRoomById(context.Background(), roomId)

	assert.Nil(t, room)
	assert.ErrorIs(t, err, errs.ErrNotFound)
	mockRepo.AssertExpectations(t)
}

func TestRoomService_GetRoomsByOwnerId_Success(t *testing.T) {
	mockRepo := new(MockRoomRepo)
	svc := NewRoomService(mockRepo)

	ownerId := uuid.New()
	expected := []domain.Room{{Id: uuid.New(), Name: "A"}, {Id: uuid.New(), Name: "B"}}

	mockRepo.On("GetRoomsByOwnerId", mock.Anything, ownerId).Return(expected, nil).Once()

	rooms, err := svc.GetRoomsByOwnerId(context.Background(), ownerId)

	assert.NoError(t, err)
	assert.Len(t, rooms, 2)
	mockRepo.AssertExpectations(t)
}

func TestRoomService_GetRoomsByOwnerId_Error(t *testing.T) {
	mockRepo := new(MockRoomRepo)
	svc := NewRoomService(mockRepo)

	ownerId := uuid.New()
	mockRepo.On("GetRoomsByOwnerId", mock.Anything, ownerId).Return(nil, errors.New("boom")).Once()

	rooms, err := svc.GetRoomsByOwnerId(context.Background(), ownerId)

	assert.Nil(t, rooms)
	assert.EqualError(t, err, "boom")
	mockRepo.AssertExpectations(t)
}

func TestRoomService_UpdateRoom_Success_NoUow(t *testing.T) {
	mockRepo := new(MockRoomRepo)
	svc := NewRoomService(mockRepo)

	roomId := uuid.New()
	existing := domain.Room{Id: roomId, Name: "Old", MaxMembers: 5, OwnerId: uuid.New()}
	newName := "New"
	newMax := 20

	mockRepo.On("GetRoomById", mock.Anything, roomId).Return(existing, nil).Once()
	mockRepo.On("UpdateRoom", mock.Anything, mock.MatchedBy(func(r domain.Room) bool {
		return r.Name == "New" && r.MaxMembers == 20
	})).Return(nil).Once()

	room, err := svc.UpdateRoom(context.Background(), roomId, structs.UpdateRoomRequest{
		Name:       &newName,
		MaxMembers: &newMax,
	})

	assert.NoError(t, err)
	assert.NotNil(t, room)
	assert.Equal(t, "New", room.Name)
	assert.Equal(t, 20, room.MaxMembers)
	mockRepo.AssertExpectations(t)
}

func TestRoomService_UpdateRoom_Success_WithUow(t *testing.T) {
	mockRepo := new(MockRoomRepo)
	mockMemberRepo := new(MockRoomMemberRepo)
	svc := NewRoomService(mockRepo, makeUow(mockRepo, mockMemberRepo))

	roomId := uuid.New()
	existing := domain.Room{Id: roomId, Name: "Old", MaxMembers: 5}
	newName := "New"

	mockRepo.On("GetRoomById", mock.Anything, roomId).Return(existing, nil).Once()
	mockRepo.On("UpdateRoom", mock.Anything, mock.MatchedBy(func(r domain.Room) bool {
		return r.Name == "New"
	})).Return(nil).Once()

	room, err := svc.UpdateRoom(context.Background(), roomId, structs.UpdateRoomRequest{Name: &newName})

	assert.NoError(t, err)
	assert.NotNil(t, room)
	assert.Equal(t, "New", room.Name)
	mockRepo.AssertExpectations(t)
	mockMemberRepo.AssertNotCalled(t, "AddMember", mock.Anything, mock.Anything)
}

func TestRoomService_UpdateRoom_NotFound(t *testing.T) {
	mockRepo := new(MockRoomRepo)
	svc := NewRoomService(mockRepo)

	roomId := uuid.New()
	mockRepo.On("GetRoomById", mock.Anything, roomId).Return(domain.Room{}, errs.ErrNotFound).Once()

	room, err := svc.UpdateRoom(context.Background(), roomId, structs.UpdateRoomRequest{})

	assert.Nil(t, room)
	assert.ErrorIs(t, err, errs.ErrNotFound)
	mockRepo.AssertExpectations(t)
}

func TestRoomService_UpdateRoom_Popped(t *testing.T) {
	mockRepo := new(MockRoomRepo)
	svc := NewRoomService(mockRepo)

	roomId := uuid.New()
	poppedAt := time.Now()
	existing := domain.Room{Id: roomId, Name: "Old", PoppedAt: &poppedAt}

	mockRepo.On("GetRoomById", mock.Anything, roomId).Return(existing, nil).Once()

	room, err := svc.UpdateRoom(context.Background(), roomId, structs.UpdateRoomRequest{})

	assert.Nil(t, room)
	assert.ErrorIs(t, err, errs.ErrRoomPopped)
	mockRepo.AssertNotCalled(t, "UpdateRoom", mock.Anything, mock.Anything)
}

func TestRoomService_UpdateRoom_RepoError(t *testing.T) {
	mockRepo := new(MockRoomRepo)
	svc := NewRoomService(mockRepo)

	roomId := uuid.New()
	existing := domain.Room{Id: roomId, Name: "Old"}
	newName := "New"

	mockRepo.On("GetRoomById", mock.Anything, roomId).Return(existing, nil).Once()
	mockRepo.On("UpdateRoom", mock.Anything, mock.Anything).Return(errors.New("db down")).Once()

	room, err := svc.UpdateRoom(context.Background(), roomId, structs.UpdateRoomRequest{Name: &newName})

	assert.Nil(t, room)
	assert.EqualError(t, err, "db down")
	mockRepo.AssertExpectations(t)
}

func TestRoomService_DeleteRoom_Success_NoUow(t *testing.T) {
	mockRepo := new(MockRoomRepo)
	svc := NewRoomService(mockRepo)

	roomId := uuid.New()
	existing := domain.Room{Id: roomId, Name: "General"}

	mockRepo.On("GetRoomById", mock.Anything, roomId).Return(existing, nil).Once()
	mockRepo.On("DeleteRoomById", mock.Anything, roomId).Return(nil).Once()

	err := svc.DeleteRoom(context.Background(), roomId)

	assert.NoError(t, err)
	mockRepo.AssertExpectations(t)
}

func TestRoomService_DeleteRoom_Success_WithUow(t *testing.T) {
	mockRepo := new(MockRoomRepo)
	mockMemberRepo := new(MockRoomMemberRepo)
	svc := NewRoomService(mockRepo, makeUow(mockRepo, mockMemberRepo))

	roomId := uuid.New()
	existing := domain.Room{Id: roomId, Name: "General"}

	mockRepo.On("GetRoomById", mock.Anything, roomId).Return(existing, nil).Once()
	mockRepo.On("DeleteRoomById", mock.Anything, roomId).Return(nil).Once()

	err := svc.DeleteRoom(context.Background(), roomId)

	assert.NoError(t, err)
	mockRepo.AssertExpectations(t)
}

func TestRoomService_DeleteRoom_Popped(t *testing.T) {
	mockRepo := new(MockRoomRepo)
	svc := NewRoomService(mockRepo)

	roomId := uuid.New()
	poppedAt := time.Now()
	existing := domain.Room{Id: roomId, PoppedAt: &poppedAt}

	mockRepo.On("GetRoomById", mock.Anything, roomId).Return(existing, nil).Once()

	err := svc.DeleteRoom(context.Background(), roomId)

	assert.ErrorIs(t, err, errs.ErrRoomPopped)
	mockRepo.AssertNotCalled(t, "DeleteRoomById", mock.Anything, mock.Anything)
}

func TestRoomService_DeleteRoom_NotFound(t *testing.T) {
	mockRepo := new(MockRoomRepo)
	svc := NewRoomService(mockRepo)

	roomId := uuid.New()
	mockRepo.On("GetRoomById", mock.Anything, roomId).Return(domain.Room{}, errs.ErrNotFound).Once()

	err := svc.DeleteRoom(context.Background(), roomId)

	assert.ErrorIs(t, err, errs.ErrNotFound)
	mockRepo.AssertNotCalled(t, "DeleteRoomById", mock.Anything, mock.Anything)
}

func TestRoomService_DeleteRoom_RepoError(t *testing.T) {
	mockRepo := new(MockRoomRepo)
	svc := NewRoomService(mockRepo)

	roomId := uuid.New()
	existing := domain.Room{Id: roomId}

	mockRepo.On("GetRoomById", mock.Anything, roomId).Return(existing, nil).Once()
	mockRepo.On("DeleteRoomById", mock.Anything, roomId).Return(errors.New("db down")).Once()

	err := svc.DeleteRoom(context.Background(), roomId)

	assert.EqualError(t, err, "db down")
	mockRepo.AssertExpectations(t)
}

func TestRoomService_PopRoom_Success(t *testing.T) {
	mockRepo := new(MockRoomRepo)
	svc := NewRoomService(mockRepo)

	roomId := uuid.New()
	existing := domain.Room{Id: roomId}

	mockRepo.On("GetRoomById", mock.Anything, roomId).Return(existing, nil).Once()
	mockRepo.On("PopRoom", mock.Anything, roomId).Return(nil).Once()

	err := svc.PopRoom(context.Background(), roomId)

	assert.NoError(t, err)
	mockRepo.AssertExpectations(t)
}

func TestRoomService_PopRoom_AlreadyPopped(t *testing.T) {
	mockRepo := new(MockRoomRepo)
	svc := NewRoomService(mockRepo)

	roomId := uuid.New()
	poppedAt := time.Now()
	existing := domain.Room{Id: roomId, PoppedAt: &poppedAt}

	mockRepo.On("GetRoomById", mock.Anything, roomId).Return(existing, nil).Once()

	err := svc.PopRoom(context.Background(), roomId)

	assert.ErrorIs(t, err, errs.ErrRoomPopped)
	mockRepo.AssertNotCalled(t, "PopRoom", mock.Anything, mock.Anything)
}

func TestRoomService_PopRoom_NotFound(t *testing.T) {
	mockRepo := new(MockRoomRepo)
	svc := NewRoomService(mockRepo)

	roomId := uuid.New()
	mockRepo.On("GetRoomById", mock.Anything, roomId).Return(domain.Room{}, errs.ErrNotFound).Once()

	err := svc.PopRoom(context.Background(), roomId)

	assert.ErrorIs(t, err, errs.ErrNotFound)
	mockRepo.AssertNotCalled(t, "PopRoom", mock.Anything, mock.Anything)
}

func TestRoomService_PopRoom_RepoError(t *testing.T) {
	mockRepo := new(MockRoomRepo)
	svc := NewRoomService(mockRepo)

	roomId := uuid.New()
	mockRepo.On("GetRoomById", mock.Anything, roomId).Return(domain.Room{Id: roomId}, nil).Once()
	mockRepo.On("PopRoom", mock.Anything, roomId).Return(errors.New("db down")).Once()

	err := svc.PopRoom(context.Background(), roomId)

	assert.EqualError(t, err, "db down")
	mockRepo.AssertExpectations(t)
}
