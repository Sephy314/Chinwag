package handler_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"testing"

	"github.com/Sephy314/chinwag/room/domain"
	"github.com/Sephy314/chinwag/room/handler"
	"github.com/Sephy314/chinwag/room/structs"
	"github.com/Sephy314/chinwag/shared/response"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/labstack/echo/v5"
	"github.com/labstack/echo/v5/echotest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type MockRoomService struct {
	mock.Mock
}

func (m *MockRoomService) CreateRoom(ctx context.Context, request structs.CreateRoomRequest) (*domain.Room, error) {
	args := m.Called(ctx, request)

	var room *domain.Room
	if args.Get(0) != nil {
		room = args.Get(0).(*domain.Room)
	}

	return room, args.Error(1)
}

func (m *MockRoomService) GetRoomById(ctx context.Context, roomId uuid.UUID) (*domain.Room, error) {
	args := m.Called(ctx, roomId)

	var room *domain.Room
	if args.Get(0) != nil {
		room = args.Get(0).(*domain.Room)
	}

	return room, args.Error(1)
}

func (m *MockRoomService) GetRoomsByOwnerId(ctx context.Context, ownerId uuid.UUID) ([]domain.Room, error) {
	args := m.Called(ctx, ownerId)

	var rooms []domain.Room
	if args.Get(0) != nil {
		rooms = args.Get(0).([]domain.Room)
	}
	return rooms, args.Error(1)
}

func (m *MockRoomService) DeleteRoom(ctx context.Context, roomId uuid.UUID) error {
	args := m.Called(ctx, roomId)
	return args.Error(0)
}

func (m *MockRoomService) UpdateRoom(ctx context.Context, roomId uuid.UUID, req structs.UpdateRoomRequest) (*domain.Room, error) {
	args := m.Called(ctx, roomId, req)

	var room *domain.Room
	if args.Get(0) != nil {
		room = args.Get(0).(*domain.Room)
	}

	return room, args.Error(1)
}

func TestRoomHandler_CreateRoom_Success(t *testing.T) {
	mockSvc := new(MockRoomService)
	mockMemberSvc := new(MockRoomMemberService)
	h := handler.NewRoomHandler(mockSvc, mockMemberSvc)

	ownerID := uuid.New()
	roomID := uuid.New()
	req := structs.CreateRoomRequest{
		Name:        "standup",
		Description: nil,
		MaxMembers:  12,
	}
	expected := &domain.Room{
		Id:         roomID,
		Name:       req.Name,
		MaxMembers: req.MaxMembers,
		OwnerId:    ownerID,
	}

	mockSvc.On("CreateRoom", mock.Anything, req).Return(expected, nil)

	c, rec := echotest.ContextConfig{
		Headers: map[string][]string{
			echo.HeaderContentType: {echo.MIMEApplicationJSON},
		},
		JSONBody: []byte(`{"name":"standup","max_members":12}`),
	}.ToContextRecorder(t)

	c.Set("user", &jwt.Token{
		Claims: jwt.MapClaims{
			"sub": ownerID.String(),
		},
	})

	err := h.CreateRoom(c)
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, http.StatusCreated, rec.Code)

	var resp response.Response[domain.Room]
	err = json.Unmarshal(rec.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Equal(t, true, resp.Success)
	assert.Equal(t, roomID, resp.Data.Id)
	assert.Equal(t, req.Name, resp.Data.Name)
	assert.Equal(t, req.MaxMembers, resp.Data.MaxMembers)
	assert.Equal(t, ownerID, resp.Data.OwnerId)

	mockSvc.AssertExpectations(t)
}

func TestRoomHandler_GetRoom_InvalidID(t *testing.T) {
	mockSvc := new(MockRoomService)
	mockMemberSvc := new(MockRoomMemberService)
	h := handler.NewRoomHandler(mockSvc, mockMemberSvc)

	rec := echotest.ContextConfig{
		PathValues: []echo.PathValue{
			{Name: "id", Value: "not-a-uuid"},
		},
	}.ServeWithHandler(t, h.GetRoom)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	mockSvc.AssertNotCalled(t, "GetRoomById", mock.Anything, mock.Anything)
}

func TestRoomHandler_ListRooms_ByOwnerId(t *testing.T) {
	mockSvc := new(MockRoomService)
	mockMemberSvc := new(MockRoomMemberService)
	h := handler.NewRoomHandler(mockSvc, mockMemberSvc)

	ownerID := uuid.New()
	rooms := []domain.Room{
		{
			Id:         uuid.New(),
			Name:       "room-1",
			MaxMembers: 8,
			OwnerId:    ownerID,
		},
	}

	mockSvc.On("GetRoomsByOwnerId", mock.Anything, ownerID).Return(rooms, nil)

	rec := echotest.ContextConfig{
		QueryValues: map[string][]string{
			"ownerId": {ownerID.String()},
		},
	}.ServeWithHandler(t, h.ListRooms)

	assert.Equal(t, http.StatusOK, rec.Code)

	var resp response.Response[[]domain.Room]
	err := json.Unmarshal(rec.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Len(t, resp.Data, 1)
	assert.Equal(t, rooms[0].Name, resp.Data[0].Name)

	mockSvc.AssertExpectations(t)
}

func TestRoomHandler_ListRooms_ByMemberId(t *testing.T) {
	mockSvc := new(MockRoomService)
	mockMemberSvc := new(MockRoomMemberService)
	h := handler.NewRoomHandler(mockSvc, mockMemberSvc)

	memberID := uuid.New()
	rooms := []domain.Room{
		{
			Id:         uuid.New(),
			Name:       "room-2",
			MaxMembers: 20,
		},
	}

	mockMemberSvc.On("GetRoomsByUserId", mock.Anything, memberID).Return(rooms, nil)

	rec := echotest.ContextConfig{
		QueryValues: map[string][]string{
			"memberId": {memberID.String()},
		},
	}.ServeWithHandler(t, h.ListRooms)

	assert.Equal(t, http.StatusOK, rec.Code)

	var resp response.Response[[]domain.Room]
	err := json.Unmarshal(rec.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Len(t, resp.Data, 1)
	assert.Equal(t, rooms[0].Name, resp.Data[0].Name)

	mockMemberSvc.AssertExpectations(t)
}

func TestRoomHandler_ListRooms_NoParam(t *testing.T) {
	mockSvc := new(MockRoomService)
	mockMemberSvc := new(MockRoomMemberService)
	h := handler.NewRoomHandler(mockSvc, mockMemberSvc)

	rec := echotest.ContextConfig{}.ServeWithHandler(t, h.ListRooms)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestRoomHandler_ListRooms_BothParams(t *testing.T) {
	mockSvc := new(MockRoomService)
	mockMemberSvc := new(MockRoomMemberService)
	h := handler.NewRoomHandler(mockSvc, mockMemberSvc)

	rec := echotest.ContextConfig{
		QueryValues: map[string][]string{
			"ownerId":  {uuid.New().String()},
			"memberId": {uuid.New().String()},
		},
	}.ServeWithHandler(t, h.ListRooms)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestRoomHandler_DeleteRoom_NotFound(t *testing.T) {
	mockSvc := new(MockRoomService)
	mockMemberSvc := new(MockRoomMemberService)
	h := handler.NewRoomHandler(mockSvc, mockMemberSvc)

	roomID := uuid.New()
	mockSvc.On("DeleteRoom", mock.Anything, roomID).Return(errors.New("not found"))

	rec := echotest.ContextConfig{
		PathValues: []echo.PathValue{
			{Name: "id", Value: roomID.String()},
		},
	}.ServeWithHandler(t, h.DeleteRoom)

	assert.Equal(t, http.StatusInternalServerError, rec.Code)

	mockSvc.AssertExpectations(t)
}

func TestRoomHandler_UpdateRoom_Success(t *testing.T) {
	mockSvc := new(MockRoomService)
	mockMemberSvc := new(MockRoomMemberService)
	h := handler.NewRoomHandler(mockSvc, mockMemberSvc)

	roomID := uuid.New()
	newName := "Updated Room"
	newMax := 50
	req := structs.UpdateRoomRequest{
		Name:       &newName,
		MaxMembers: &newMax,
	}
	expected := &domain.Room{
		Id:         roomID,
		Name:       "Updated Room",
		MaxMembers: 50,
		OwnerId:    uuid.New(),
	}

	mockSvc.On("UpdateRoom", mock.Anything, roomID, req).Return(expected, nil)

	c, rec := echotest.ContextConfig{
		PathValues: []echo.PathValue{
			{Name: "id", Value: roomID.String()},
		},
		Headers: map[string][]string{
			echo.HeaderContentType: {echo.MIMEApplicationJSON},
		},
		JSONBody: []byte(`{"name":"Updated Room","max_members":50}`),
	}.ToContextRecorder(t)

	err := h.UpdateRoom(c)
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, http.StatusOK, rec.Code)

	var resp response.Response[domain.Room]
	err = json.Unmarshal(rec.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Equal(t, true, resp.Success)
	assert.Equal(t, "Updated Room", resp.Data.Name)
	assert.Equal(t, 50, resp.Data.MaxMembers)

	mockSvc.AssertExpectations(t)
}

func TestRoomHandler_UpdateRoom_InvalidID(t *testing.T) {
	mockSvc := new(MockRoomService)
	mockMemberSvc := new(MockRoomMemberService)
	h := handler.NewRoomHandler(mockSvc, mockMemberSvc)

	c, rec := echotest.ContextConfig{
		PathValues: []echo.PathValue{
			{Name: "id", Value: "not-a-uuid"},
		},
		Headers: map[string][]string{
			echo.HeaderContentType: {echo.MIMEApplicationJSON},
		},
		JSONBody: []byte(`{"name":"test"}`),
	}.ToContextRecorder(t)

	err := h.UpdateRoom(c)
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	mockSvc.AssertNotCalled(t, "UpdateRoom", mock.Anything, mock.Anything, mock.Anything)
}

func TestRoomHandler_UpdateRoom_NotFound(t *testing.T) {
	mockSvc := new(MockRoomService)
	mockMemberSvc := new(MockRoomMemberService)
	h := handler.NewRoomHandler(mockSvc, mockMemberSvc)

	roomID := uuid.New()
	mockSvc.On("UpdateRoom", mock.Anything, roomID, mock.Anything).Return(nil, errors.New("not found"))

	c, rec := echotest.ContextConfig{
		PathValues: []echo.PathValue{
			{Name: "id", Value: roomID.String()},
		},
		Headers: map[string][]string{
			echo.HeaderContentType: {echo.MIMEApplicationJSON},
		},
		JSONBody: []byte(`{"name":"test"}`),
	}.ToContextRecorder(t)

	err := h.UpdateRoom(c)
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
	mockSvc.AssertExpectations(t)
}
