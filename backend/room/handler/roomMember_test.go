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

func (m *MockRoomMemberService) SetUserRole(ctx context.Context, userId uuid.UUID, roomId uuid.UUID, role domain.Role) error {
	args := m.Called(ctx, userId, roomId, role)
	return args.Error(0)
}

func TestRoomMemberHandler_AddMember_Success(t *testing.T) {
	mockSvc := new(MockRoomMemberService)
	mockRoomSvc := new(MockRoomService)
	h := handler.NewRoomMemberHandler(mockSvc, mockRoomSvc)

	userID := uuid.New()
	roomID := uuid.New()
	role := domain.ADMIN
	req := structs.RoomUser{
		UserId: userID,
		RoomId: roomID,
		Role:   &role,
	}

	mockSvc.On("HasManagerPermission", mock.Anything, mock.Anything, roomID).Return(true, nil)
	mockSvc.On("InviteUser", mock.Anything, req).Return(nil)

	c, rec := echotest.ContextConfig{
		PathValues: []echo.PathValue{
			{Name: "roomId", Value: roomID.String()},
		},
		Headers: map[string][]string{
			echo.HeaderContentType: {echo.MIMEApplicationJSON},
		},
		JSONBody: []byte(`{"userId":"` + userID.String() + `","role":1}`),
	}.ToContextRecorder(t)

	c.Set("user", &jwt.Token{
		Claims: jwt.MapClaims{
			"sub": userID.String(),
		},
	})

	err := h.AddMember(c)
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, http.StatusCreated, rec.Code)

	var resp response.Response[any]
	err = json.Unmarshal(rec.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Equal(t, true, resp.Success)

	mockSvc.AssertExpectations(t)
}

func TestRoomMemberHandler_AddMember_InvalidRoomId(t *testing.T) {
	mockSvc := new(MockRoomMemberService)
	mockRoomSvc := new(MockRoomService)
	h := handler.NewRoomMemberHandler(mockSvc, mockRoomSvc)

	rec := echotest.ContextConfig{
		PathValues: []echo.PathValue{
			{Name: "roomId", Value: "not-a-uuid"},
		},
	}.ServeWithHandler(t, h.AddMember)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	mockSvc.AssertNotCalled(t, "InviteUser", mock.Anything, mock.Anything)
}

func TestRoomMemberHandler_RemoveMember_Success(t *testing.T) {
	mockSvc := new(MockRoomMemberService)
	mockRoomSvc := new(MockRoomService)
	h := handler.NewRoomMemberHandler(mockSvc, mockRoomSvc)

	userID := uuid.New()
	roomID := uuid.New()
	req := structs.RoomUser{
		UserId: userID,
		RoomId: roomID,
	}

	mockSvc.On("KickUser", mock.Anything, req).Return(nil)

	rec := echotest.ContextConfig{
		PathValues: []echo.PathValue{
			{Name: "roomId", Value: roomID.String()},
			{Name: "userId", Value: userID.String()},
		},
	}.ServeWithHandler(t, h.RemoveMember)

	assert.Equal(t, http.StatusOK, rec.Code)

	var resp response.Response[any]
	err := json.Unmarshal(rec.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Equal(t, true, resp.Success)

	mockSvc.AssertExpectations(t)
}

func TestRoomMemberHandler_RemoveMember_InvalidRoomId(t *testing.T) {
	mockSvc := new(MockRoomMemberService)
	mockRoomSvc := new(MockRoomService)
	h := handler.NewRoomMemberHandler(mockSvc, mockRoomSvc)

	rec := echotest.ContextConfig{
		PathValues: []echo.PathValue{
			{Name: "roomId", Value: "not-a-uuid"},
			{Name: "userId", Value: uuid.New().String()},
		},
	}.ServeWithHandler(t, h.RemoveMember)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	mockSvc.AssertNotCalled(t, "KickUser", mock.Anything, mock.Anything)
}

func TestRoomMemberHandler_RemoveMember_InvalidUserId(t *testing.T) {
	mockSvc := new(MockRoomMemberService)
	mockRoomSvc := new(MockRoomService)
	h := handler.NewRoomMemberHandler(mockSvc, mockRoomSvc)

	rec := echotest.ContextConfig{
		PathValues: []echo.PathValue{
			{Name: "roomId", Value: uuid.New().String()},
			{Name: "userId", Value: "not-a-uuid"},
		},
	}.ServeWithHandler(t, h.RemoveMember)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	mockSvc.AssertNotCalled(t, "KickUser", mock.Anything, mock.Anything)
}

func TestRoomMemberHandler_ListMembers_Success(t *testing.T) {
	mockSvc := new(MockRoomMemberService)
	mockRoomSvc := new(MockRoomService)
	h := handler.NewRoomMemberHandler(mockSvc, mockRoomSvc)

	roomID := uuid.New()
	members := []domain.RoomMember{
		{UserId: uuid.New(), RoomId: roomID, Role: domain.MEMBER},
		{UserId: uuid.New(), RoomId: roomID, Role: domain.ADMIN},
	}

	mockSvc.On("GetUserByRoomId", mock.Anything, roomID).Return(members, nil)

	rec := echotest.ContextConfig{
		PathValues: []echo.PathValue{
			{Name: "roomId", Value: roomID.String()},
		},
	}.ServeWithHandler(t, h.ListMembers)

	assert.Equal(t, http.StatusOK, rec.Code)

	var resp response.Response[[]domain.RoomMember]
	err := json.Unmarshal(rec.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Len(t, resp.Data, 2)

	mockSvc.AssertExpectations(t)
}

func TestRoomMemberHandler_ListMembers_InvalidRoomId(t *testing.T) {
	mockSvc := new(MockRoomMemberService)
	mockRoomSvc := new(MockRoomService)
	h := handler.NewRoomMemberHandler(mockSvc, mockRoomSvc)

	rec := echotest.ContextConfig{
		PathValues: []echo.PathValue{
			{Name: "roomId", Value: "not-a-uuid"},
		},
	}.ServeWithHandler(t, h.ListMembers)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	mockSvc.AssertNotCalled(t, "GetUserByRoomId", mock.Anything, mock.Anything)
}

func TestRoomMemberHandler_GetMember_Success(t *testing.T) {
	mockSvc := new(MockRoomMemberService)
	mockRoomSvc := new(MockRoomService)
	h := handler.NewRoomMemberHandler(mockSvc, mockRoomSvc)

	userID := uuid.New()
	roomID := uuid.New()
	member := &domain.RoomMember{
		UserId: userID,
		RoomId: roomID,
		Role:   domain.MEMBER,
	}

	mockSvc.On("GetUserByRoomIdAndUserId", mock.Anything, userID, roomID).Return(member, nil)

	rec := echotest.ContextConfig{
		PathValues: []echo.PathValue{
			{Name: "roomId", Value: roomID.String()},
			{Name: "userId", Value: userID.String()},
		},
	}.ServeWithHandler(t, h.GetMember)

	assert.Equal(t, http.StatusOK, rec.Code)

	var resp response.Response[domain.RoomMember]
	err := json.Unmarshal(rec.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Equal(t, true, resp.Success)
	assert.Equal(t, userID, resp.Data.UserId)
	assert.Equal(t, roomID, resp.Data.RoomId)

	mockSvc.AssertExpectations(t)
}

func TestRoomMemberHandler_GetMember_InvalidRoomId(t *testing.T) {
	mockSvc := new(MockRoomMemberService)
	mockRoomSvc := new(MockRoomService)
	h := handler.NewRoomMemberHandler(mockSvc, mockRoomSvc)

	rec := echotest.ContextConfig{
		PathValues: []echo.PathValue{
			{Name: "roomId", Value: "not-a-uuid"},
			{Name: "userId", Value: uuid.New().String()},
		},
	}.ServeWithHandler(t, h.GetMember)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	mockSvc.AssertNotCalled(t, "GetUserByRoomIdAndUserId", mock.Anything, mock.Anything, mock.Anything)
}

func TestRoomMemberHandler_GetMember_InvalidUserId(t *testing.T) {
	mockSvc := new(MockRoomMemberService)
	mockRoomSvc := new(MockRoomService)
	h := handler.NewRoomMemberHandler(mockSvc, mockRoomSvc)

	rec := echotest.ContextConfig{
		PathValues: []echo.PathValue{
			{Name: "roomId", Value: uuid.New().String()},
			{Name: "userId", Value: "not-a-uuid"},
		},
	}.ServeWithHandler(t, h.GetMember)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	mockSvc.AssertNotCalled(t, "GetUserByRoomIdAndUserId", mock.Anything, mock.Anything, mock.Anything)
}

func TestRoomMemberHandler_RemoveMember_NotFound(t *testing.T) {
	mockSvc := new(MockRoomMemberService)
	mockRoomSvc := new(MockRoomService)
	h := handler.NewRoomMemberHandler(mockSvc, mockRoomSvc)

	userID := uuid.New()
	roomID := uuid.New()
	req := structs.RoomUser{
		UserId: userID,
		RoomId: roomID,
	}

	mockSvc.On("KickUser", mock.Anything, req).Return(errors.New("not found"))

	rec := echotest.ContextConfig{
		PathValues: []echo.PathValue{
			{Name: "roomId", Value: roomID.String()},
			{Name: "userId", Value: userID.String()},
		},
	}.ServeWithHandler(t, h.RemoveMember)

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
	mockSvc.AssertExpectations(t)
}
