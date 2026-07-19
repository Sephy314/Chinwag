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

type MockRoomRepo struct {
	mock.Mock
}

func (m *MockRoomRepo) CreateRoom(ctx context.Context, room domain.Room) error {
	args := m.Called(ctx, room)
	return args.Error(0)
}

func (m *MockRoomRepo) GetRoomById(ctx context.Context, roomId uuid.UUID) (domain.Room, error) {
	args := m.Called(ctx, roomId)
	return args.Get(0).(domain.Room), args.Error(1)
}

func (m *MockRoomRepo) GetRoomsByOwnerId(ctx context.Context, ownerId uuid.UUID) ([]domain.Room, error) {
	args := m.Called(ctx, ownerId)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]domain.Room), args.Error(1)
}

func (m *MockRoomRepo) DeleteRoomById(ctx context.Context, roomId uuid.UUID) error {
	args := m.Called(ctx, roomId)
	return args.Error(0)
}

func (m *MockRoomRepo) UpdateRoom(ctx context.Context, room domain.Room) error {
	args := m.Called(ctx, room)
	return args.Error(0)
}

func TestCreateRoom_Success(t *testing.T) {
	mockRepo := new(MockRoomRepo)
	ownerId := uuid.New()
	ctx := context.WithValue(context.Background(), "ownerId", ownerId)

	req := structs.CreateRoomRequest{
		Name:        "Test Room",
		Description: nil,
		MaxMembers:  10,
	}

	mockRepo.On("CreateRoom", ctx, mock.MatchedBy(func(room domain.Room) bool {
		return room.Name == req.Name && room.MaxMembers == req.MaxMembers && room.OwnerId == ownerId
	})).Return(nil)

	service := NewRoomService(mockRepo)
	result, err := service.CreateRoom(ctx, req)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, req.Name, result.Name)
	assert.Equal(t, req.MaxMembers, result.MaxMembers)
	assert.Equal(t, ownerId, result.OwnerId)
	mockRepo.AssertExpectations(t)
}

func TestCreateRoom_Failed(t *testing.T) {
	mockRepo := new(MockRoomRepo)
	ownerId := uuid.New()
	ctx := context.WithValue(context.Background(), "ownerId", ownerId)

	req := structs.CreateRoomRequest{
		Name:        "Test Room",
		Description: nil,
		MaxMembers:  10,
	}

	mockRepo.On("CreateRoom", ctx, mock.MatchedBy(func(room domain.Room) bool {
		return room.Name == req.Name && room.OwnerId == ownerId
	})).Return(errors.New("database error"))

	service := NewRoomService(mockRepo)
	result, err := service.CreateRoom(ctx, req)

	assert.Error(t, err)
	assert.Equal(t, "database error", err.Error())
	assert.NotNil(t, result)
	mockRepo.AssertExpectations(t)
}

func TestCreateRoom_Unauthorized(t *testing.T) {
	mockRepo := new(MockRoomRepo)
	ctx := context.Background()

	req := structs.CreateRoomRequest{
		Name:        "Test Room",
		Description: nil,
		MaxMembers:  10,
	}

	service := NewRoomService(mockRepo)
	result, err := service.CreateRoom(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, result)
}

func TestGetRoomById_Success(t *testing.T) {
	mockRepo := new(MockRoomRepo)
	ctx := context.Background()

	roomId := uuid.New()
	expectedRoom := domain.Room{
		Id:          roomId,
		Name:        "Test Room",
		Description: nil,
		MaxMembers:  10,
		OwnerId:     uuid.New(),
	}

	mockRepo.On("GetRoomById", ctx, roomId).Return(expectedRoom, nil)

	service := NewRoomService(mockRepo)
	result, err := service.GetRoomById(ctx, roomId)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, expectedRoom.Id, result.Id)
	assert.Equal(t, expectedRoom.Name, result.Name)
	mockRepo.AssertExpectations(t)
}

func TestGetRoomById_NotFound(t *testing.T) {
	mockRepo := new(MockRoomRepo)
	ctx := context.Background()

	roomId := uuid.New()

	mockRepo.On("GetRoomById", ctx, roomId).Return(domain.Room{}, errors.New("not found"))

	service := NewRoomService(mockRepo)
	result, err := service.GetRoomById(ctx, roomId)

	assert.Error(t, err)
	assert.Equal(t, "not found", err.Error())
	assert.Nil(t, result)
	mockRepo.AssertExpectations(t)
}

func TestGetRoomsByOwnerId_Success(t *testing.T) {
	mockRepo := new(MockRoomRepo)
	ctx := context.Background()

	ownerId := uuid.New()
	expectedRooms := []domain.Room{
		{
			Id:         uuid.New(),
			Name:       "Room 1",
			MaxMembers: 10,
			OwnerId:    ownerId,
		},
		{
			Id:         uuid.New(),
			Name:       "Room 2",
			MaxMembers: 20,
			OwnerId:    ownerId,
		},
	}

	mockRepo.On("GetRoomsByOwnerId", ctx, ownerId).Return(expectedRooms, nil)

	service := NewRoomService(mockRepo)
	result, err := service.GetRoomsByOwnerId(ctx, ownerId)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, len(expectedRooms), len(result))
	assert.Equal(t, expectedRooms[0].Name, (result)[0].Name)
	mockRepo.AssertExpectations(t)
}

func TestGetRoomsByOwnerId_Empty(t *testing.T) {
	mockRepo := new(MockRoomRepo)
	ctx := context.Background()

	ownerId := uuid.New()

	mockRepo.On("GetRoomsByOwnerId", ctx, ownerId).Return([]domain.Room{}, nil)

	service := NewRoomService(mockRepo)
	result, err := service.GetRoomsByOwnerId(ctx, ownerId)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, 0, len(result))
	mockRepo.AssertExpectations(t)
}

func TestGetRoomsByOwnerId_Failed(t *testing.T) {
	mockRepo := new(MockRoomRepo)
	ctx := context.Background()

	ownerId := uuid.New()

	mockRepo.On("GetRoomsByOwnerId", ctx, ownerId).Return(nil, errors.New("database error"))

	service := NewRoomService(mockRepo)
	result, err := service.GetRoomsByOwnerId(ctx, ownerId)

	assert.Error(t, err)
	assert.Equal(t, "database error", err.Error())
	assert.Nil(t, result)
	mockRepo.AssertExpectations(t)
}

func TestDeleteRoom_Success(t *testing.T) {
	mockRepo := new(MockRoomRepo)
	ctx := context.Background()

	roomId := uuid.New()

	mockRepo.On("GetRoomById", ctx, roomId).Return(domain.Room{Id: roomId}, nil)
	mockRepo.On("DeleteRoomById", ctx, roomId).Return(nil)

	service := NewRoomService(mockRepo)
	err := service.DeleteRoom(ctx, roomId)

	assert.NoError(t, err)
	mockRepo.AssertExpectations(t)
}

func TestDeleteRoom_NotFound(t *testing.T) {
	mockRepo := new(MockRoomRepo)
	ctx := context.Background()

	roomId := uuid.New()

	mockRepo.On("GetRoomById", ctx, roomId).Return(domain.Room{}, errors.New("not found"))

	service := NewRoomService(mockRepo)
	err := service.DeleteRoom(ctx, roomId)

	assert.Error(t, err)
	assert.Equal(t, "not found", err.Error())
	mockRepo.AssertExpectations(t)
}

func TestDeleteRoom_Popped(t *testing.T) {
	mockRepo := new(MockRoomRepo)
	ctx := context.Background()

	roomId := uuid.New()
	now := time.Now()

	mockRepo.On("GetRoomById", ctx, roomId).Return(domain.Room{
		Id:       roomId,
		PoppedAt: &now,
	}, nil)

	service := NewRoomService(mockRepo)
	err := service.DeleteRoom(ctx, roomId)

	assert.Error(t, err)
	assert.Equal(t, errs.ErrRoomPopped, err)
	mockRepo.AssertExpectations(t)
}

func TestCreateRoom_DuplicateRoom(t *testing.T) {
	mockRepo := new(MockRoomRepo)
	ownerId := uuid.New()
	ctx := context.WithValue(context.Background(), "ownerId", ownerId)

	req := structs.CreateRoomRequest{
		Name:        "Duplicate Room",
		Description: nil,
		MaxMembers:  10,
	}

	mockRepo.On("CreateRoom", ctx, mock.MatchedBy(func(room domain.Room) bool {
		return room.Name == req.Name && room.OwnerId == ownerId
	})).Return(errors.New("duplicate key value violates unique constraint"))

	service := NewRoomService(mockRepo)
	result, err := service.CreateRoom(ctx, req)

	assert.Error(t, err)
	assert.Equal(t, "duplicate key value violates unique constraint", err.Error())
	assert.NotNil(t, result)
	mockRepo.AssertExpectations(t)
}

func TestUpdateRoom_Success(t *testing.T) {
	mockRepo := new(MockRoomRepo)
	ctx := context.Background()

	roomId := uuid.New()
	existingRoom := domain.Room{
		Id:          roomId,
		Name:        "Old Name",
		Description: nil,
		MaxMembers:  10,
		OwnerId:     uuid.New(),
	}

	newName := "New Name"
	req := structs.UpdateRoomRequest{
		Name: &newName,
	}

	mockRepo.On("GetRoomById", ctx, roomId).Return(existingRoom, nil)
	mockRepo.On("UpdateRoom", ctx, mock.MatchedBy(func(room domain.Room) bool {
		return room.Name == "New Name" && room.Id == roomId
	})).Return(nil)

	service := NewRoomService(mockRepo)
	result, err := service.UpdateRoom(ctx, roomId, req)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, "New Name", result.Name)
	assert.Equal(t, roomId, result.Id)
	mockRepo.AssertExpectations(t)
}

func TestUpdateRoom_NotFound(t *testing.T) {
	mockRepo := new(MockRoomRepo)
	ctx := context.Background()

	roomId := uuid.New()

	mockRepo.On("GetRoomById", ctx, roomId).Return(domain.Room{}, errors.New("not found"))

	service := NewRoomService(mockRepo)
	result, err := service.UpdateRoom(ctx, roomId, structs.UpdateRoomRequest{})

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Equal(t, "not found", err.Error())
	mockRepo.AssertExpectations(t)
}

func TestUpdateRoom_UpdateFailed(t *testing.T) {
	mockRepo := new(MockRoomRepo)
	ctx := context.Background()

	roomId := uuid.New()
	existingRoom := domain.Room{
		Id:         roomId,
		Name:       "Old Name",
		MaxMembers: 10,
		OwnerId:    uuid.New(),
	}

	newName := "New Name"
	req := structs.UpdateRoomRequest{
		Name: &newName,
	}

	mockRepo.On("GetRoomById", ctx, roomId).Return(existingRoom, nil)
	mockRepo.On("UpdateRoom", ctx, mock.Anything).Return(errors.New("database error"))

	service := NewRoomService(mockRepo)
	result, err := service.UpdateRoom(ctx, roomId, req)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Equal(t, "database error", err.Error())
	mockRepo.AssertExpectations(t)
}

func TestUpdateRoom_PartialUpdate(t *testing.T) {
	mockRepo := new(MockRoomRepo)
	ctx := context.Background()

	roomId := uuid.New()
	existingRoom := domain.Room{
		Id:          roomId,
		Name:        "Old Name",
		Description: nil,
		MaxMembers:  10,
		OwnerId:     uuid.New(),
	}

	newMax := 50
	req := structs.UpdateRoomRequest{
		MaxMembers: &newMax,
	}

	mockRepo.On("GetRoomById", ctx, roomId).Return(existingRoom, nil)
	mockRepo.On("UpdateRoom", ctx, mock.MatchedBy(func(room domain.Room) bool {
		return room.Name == "Old Name" && room.MaxMembers == 50
	})).Return(nil)

	service := NewRoomService(mockRepo)
	result, err := service.UpdateRoom(ctx, roomId, req)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, "Old Name", result.Name)
	assert.Equal(t, 50, result.MaxMembers)
	mockRepo.AssertExpectations(t)
}

func TestUpdateRoom_Popped(t *testing.T) {
	mockRepo := new(MockRoomRepo)
	ctx := context.Background()

	roomId := uuid.New()
	now := time.Now()
	existingRoom := domain.Room{
		Id:         roomId,
		Name:       "Popped Room",
		MaxMembers: 10,
		OwnerId:    uuid.New(),
		PoppedAt:   &now,
	}

	newName := "New Name"
	req := structs.UpdateRoomRequest{
		Name: &newName,
	}

	mockRepo.On("GetRoomById", ctx, roomId).Return(existingRoom, nil)

	service := NewRoomService(mockRepo)
	result, err := service.UpdateRoom(ctx, roomId, req)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Equal(t, errs.ErrRoomPopped, err)
	mockRepo.AssertExpectations(t)
}

func TestCreateRoom_WithCustomPopAt(t *testing.T) {
	mockRepo := new(MockRoomRepo)
	ownerId := uuid.New()
	ctx := context.WithValue(context.Background(), "ownerId", ownerId)

	customPopAt := time.Now().Add(48 * time.Hour)
	req := structs.CreateRoomRequest{
		Name:        "Custom Pop Room",
		Description: nil,
		MaxMembers:  10,
		PopAt:       &customPopAt,
	}

	mockRepo.On("CreateRoom", ctx, mock.MatchedBy(func(room domain.Room) bool {
		return room.Name == req.Name && room.PopAt.Equal(customPopAt)
	})).Return(nil)

	service := NewRoomService(mockRepo)
	result, err := service.CreateRoom(ctx, req)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, req.Name, result.Name)
	assert.True(t, result.PopAt.Equal(customPopAt))
	mockRepo.AssertExpectations(t)
}
