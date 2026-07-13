package service

import (
	"context"
	"errors"
	"testing"

	"github.com/Sephy314/chinwag/room/domain"
	"github.com/Sephy314/chinwag/room/structs"
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

func TestCreateRoom_Success(t *testing.T) {
	mockRepo := new(MockRoomRepo)
	ctx := context.Background()

	req := structs.CreateRoomRequest{
		Name:        "Test Room",
		Description: nil,
		MaxMembers:  10,
		OwnerId:     uuid.New(),
	}

	mockRepo.On("CreateRoom", ctx, mock.MatchedBy(func(room domain.Room) bool {
		return room.Name == req.Name && room.MaxMembers == req.MaxMembers && room.OwnerId == req.OwnerId
	})).Return(nil)

	service := NewRoomService(mockRepo)
	result, err := service.CreateRoom(ctx, req)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, req.Name, result.Name)
	assert.Equal(t, req.MaxMembers, result.MaxMembers)
	mockRepo.AssertExpectations(t)
}

func TestCreateRoom_Failed(t *testing.T) {
	mockRepo := new(MockRoomRepo)
	ctx := context.Background()

	req := structs.CreateRoomRequest{
		Name:        "Test Room",
		Description: nil,
		MaxMembers:  10,
		OwnerId:     uuid.New(),
	}

	mockRepo.On("CreateRoom", ctx, mock.MatchedBy(func(room domain.Room) bool {
		return room.Name == req.Name && room.OwnerId == req.OwnerId
	})).Return(errors.New("database error"))

	service := NewRoomService(mockRepo)
	result, err := service.CreateRoom(ctx, req)

	assert.Error(t, err)
	assert.Equal(t, "database error", err.Error())
	assert.NotNil(t, result)
	mockRepo.AssertExpectations(t)
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

	mockRepo.On("DeleteRoomById", ctx, roomId).Return(errors.New("not found"))

	service := NewRoomService(mockRepo)
	err := service.DeleteRoom(ctx, roomId)

	assert.Error(t, err)
	assert.Equal(t, "not found", err.Error())
	mockRepo.AssertExpectations(t)
}

func TestCreateRoom_DuplicateRoom(t *testing.T) {
	mockRepo := new(MockRoomRepo)
	ctx := context.Background()

	req := structs.CreateRoomRequest{
		Name:        "Duplicate Room",
		Description: nil,
		MaxMembers:  10,
		OwnerId:     uuid.New(),
	}

	mockRepo.On("CreateRoom", ctx, mock.MatchedBy(func(room domain.Room) bool {
		return room.Name == req.Name && room.OwnerId == req.OwnerId
	})).Return(errors.New("duplicate key value violates unique constraint"))

	service := NewRoomService(mockRepo)
	result, err := service.CreateRoom(ctx, req)

	assert.Error(t, err)
	assert.Equal(t, "duplicate key value violates unique constraint", err.Error())
	assert.NotNil(t, result)
	mockRepo.AssertExpectations(t)
}
