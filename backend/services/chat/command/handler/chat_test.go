package handler_test

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/Sephy314/chinwag/backend/services/chat/command/domain"
	"github.com/Sephy314/chinwag/backend/services/chat/command/handler"
	"github.com/Sephy314/chinwag/backend/services/chat/command/shared/response"
	"github.com/Sephy314/chinwag/backend/services/chat/command/structs"
	sharedauth "github.com/Sephy314/chinwag/backend/shared/auth"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/labstack/echo/v5"
	"github.com/labstack/echo/v5/echotest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type MockChatService struct {
	mock.Mock
}

func (m *MockChatService) CreateMessage(ctx context.Context, roomId uuid.UUID, req structs.CreateMessageRequest) (*structs.MessageResponse, error) {
	args := m.Called(ctx, roomId, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*structs.MessageResponse), args.Error(1)
}

func (m *MockChatService) UpdateMessage(ctx context.Context, messageId uuid.UUID, userId uuid.UUID, req structs.UpdateMessageRequest) (*structs.MessageResponse, error) {
	args := m.Called(ctx, messageId, userId, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*structs.MessageResponse), args.Error(1)
}

func (m *MockChatService) DeleteMessage(ctx context.Context, messageId uuid.UUID, userId uuid.UUID) error {
	args := m.Called(ctx, messageId, userId)
	return args.Error(0)
}

func (m *MockChatService) AdminDeleteMessage(ctx context.Context, messageId uuid.UUID) error {
	args := m.Called(ctx, messageId)
	return args.Error(0)
}

func TestChatHandler_Health(t *testing.T) {
	mockSvc := new(MockChatService)
	h := handler.NewChatHandler(mockSvc)

	c, rec := echotest.ContextConfig{}.ToContextRecorder(t)
	err := h.Health(c)
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, http.StatusOK, rec.Code)

	var resp response.Response[any]
	err = json.Unmarshal(rec.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.True(t, resp.Success)
}

func TestChatHandler_CreateMessage_Success(t *testing.T) {
	mockSvc := new(MockChatService)
	h := handler.NewChatHandler(mockSvc)

	roomID := uuid.New()
	userID := uuid.New()
	messageID := uuid.New()
	now := time.Now()

	expected := &structs.MessageResponse{
		Id:          messageID.String(),
		RoomId:      roomID.String(),
		AuthorId:    userID.String(),
		AuthorName:  "testuser",
		MessageType: domain.MessageTypeTEXT,
		Content:     "Hello, world!",
		CreatedAt:   now,
	}

	mockSvc.On("CreateMessage", mock.Anything, roomID, structs.CreateMessageRequest{
		Id:          messageID,
		MessageType: domain.MessageTypeTEXT,
		Content:     "Hello, world!",
	}).Return(expected, nil)

	c, rec := echotest.ContextConfig{
		PathValues: []echo.PathValue{
			{Name: "roomId", Value: roomID.String()},
		},
		Headers: map[string][]string{
			echo.HeaderContentType: {echo.MIMEApplicationJSON},
		},
		JSONBody: []byte(`{"id":"` + messageID.String() + `","message_type":0,"content":"Hello, world!"}`),
	}.ToContextRecorder(t)
	c.Set(sharedauth.ClaimsContextKey, &sharedauth.Claims{RegisteredClaims: jwt.RegisteredClaims{Subject: userID.String()}})

	err := h.CreateMessage(c)
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, http.StatusCreated, rec.Code)

	var resp response.Response[structs.MessageResponse]
	err = json.Unmarshal(rec.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.True(t, resp.Success)
	assert.Equal(t, "Hello, world!", resp.Data.Content)
	assert.Equal(t, "testuser", resp.Data.AuthorName)

	mockSvc.AssertExpectations(t)
}

func TestChatHandler_CreateMessage_MissingMessageID(t *testing.T) {
	mockSvc := new(MockChatService)
	h := handler.NewChatHandler(mockSvc)

	userID := uuid.New()
	roomID := uuid.New()
	c, rec := echotest.ContextConfig{
		PathValues: []echo.PathValue{{Name: "roomId", Value: roomID.String()}},
		Headers:    map[string][]string{echo.HeaderContentType: {echo.MIMEApplicationJSON}},
		JSONBody:   []byte(`{"message_type":0,"content":"Hello"}`),
	}.ToContextRecorder(t)
	c.Set(sharedauth.ClaimsContextKey, &sharedauth.Claims{RegisteredClaims: jwt.RegisteredClaims{Subject: userID.String()}})

	err := h.CreateMessage(c)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	mockSvc.AssertNotCalled(t, "CreateMessage", mock.Anything, mock.Anything, mock.Anything)
}

func TestChatHandler_CreateMessage_InvalidRoomID(t *testing.T) {
	mockSvc := new(MockChatService)
	h := handler.NewChatHandler(mockSvc)

	rec := echotest.ContextConfig{
		PathValues: []echo.PathValue{
			{Name: "roomId", Value: "not-a-uuid"},
		},
		Headers: map[string][]string{
			echo.HeaderContentType: {echo.MIMEApplicationJSON},
		},
		JSONBody: []byte(`{"message_type":0,"content":"Hello"}`),
	}.ServeWithHandler(t, h.CreateMessage)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	mockSvc.AssertNotCalled(t, "CreateMessage", mock.Anything, mock.Anything, mock.Anything)
}

func TestChatHandler_CreateMessage_InvalidJSON(t *testing.T) {
	mockSvc := new(MockChatService)
	h := handler.NewChatHandler(mockSvc)

	userID := uuid.New()
	roomID := uuid.New()

	c, rec := echotest.ContextConfig{
		PathValues: []echo.PathValue{
			{Name: "roomId", Value: roomID.String()},
		},
		Headers: map[string][]string{
			echo.HeaderContentType: {echo.MIMEApplicationJSON},
		},
		JSONBody: []byte(`{invalid json`),
	}.ToContextRecorder(t)
	c.Set(sharedauth.ClaimsContextKey, &sharedauth.Claims{RegisteredClaims: jwt.RegisteredClaims{Subject: userID.String()}})

	err := h.CreateMessage(c)
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	mockSvc.AssertNotCalled(t, "CreateMessage", mock.Anything, mock.Anything, mock.Anything)
}

func TestChatHandler_CreateMessage_Unauthorized(t *testing.T) {
	mockSvc := new(MockChatService)
	h := handler.NewChatHandler(mockSvc)

	roomID := uuid.New()

	rec := echotest.ContextConfig{
		PathValues: []echo.PathValue{
			{Name: "roomId", Value: roomID.String()},
		},
		Headers: map[string][]string{
			echo.HeaderContentType: {echo.MIMEApplicationJSON},
		},
		JSONBody: []byte(`{"message_type":0,"content":"Hello"}`),
	}.ServeWithHandler(t, h.CreateMessage)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
	mockSvc.AssertNotCalled(t, "CreateMessage", mock.Anything, mock.Anything, mock.Anything)
}

func TestChatHandler_UpdateMessage_Success(t *testing.T) {
	mockSvc := new(MockChatService)
	h := handler.NewChatHandler(mockSvc)

	userID := uuid.New()
	messageID := uuid.New()
	roomID := uuid.New()
	now := time.Now()

	expected := &structs.MessageResponse{
		Id:          messageID.String(),
		RoomId:      roomID.String(),
		AuthorId:    userID.String(),
		AuthorName:  "testuser",
		MessageType: domain.MessageTypeTEXT,
		Content:     "Updated content",
		CreatedAt:   now,
	}

	mockSvc.On("UpdateMessage", mock.Anything, messageID, userID, structs.UpdateMessageRequest{
		Content: strPtr("Updated content"),
	}).Return(expected, nil)

	c, rec := echotest.ContextConfig{
		PathValues: []echo.PathValue{
			{Name: "roomId", Value: roomID.String()},
			{Name: "messageId", Value: messageID.String()},
		},
		Headers: map[string][]string{
			echo.HeaderContentType: {echo.MIMEApplicationJSON},
		},
		JSONBody: []byte(`{"content":"Updated content"}`),
	}.ToContextRecorder(t)
	c.Set(sharedauth.ClaimsContextKey, &sharedauth.Claims{RegisteredClaims: jwt.RegisteredClaims{Subject: userID.String()}})

	err := h.UpdateMessage(c)
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, http.StatusOK, rec.Code)

	var resp response.Response[structs.MessageResponse]
	err = json.Unmarshal(rec.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.True(t, resp.Success)
	assert.Equal(t, "Updated content", resp.Data.Content)

	mockSvc.AssertExpectations(t)
}

func TestChatHandler_UpdateMessage_InvalidMessageID(t *testing.T) {
	mockSvc := new(MockChatService)
	h := handler.NewChatHandler(mockSvc)

	rec := echotest.ContextConfig{
		PathValues: []echo.PathValue{
			{Name: "roomId", Value: uuid.New().String()},
			{Name: "messageId", Value: "not-a-uuid"},
		},
		Headers: map[string][]string{
			echo.HeaderContentType: {echo.MIMEApplicationJSON},
		},
		JSONBody: []byte(`{"content":"test"}`),
	}.ServeWithHandler(t, h.UpdateMessage)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	mockSvc.AssertNotCalled(t, "UpdateMessage", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

func TestChatHandler_DeleteMessage_Success(t *testing.T) {
	mockSvc := new(MockChatService)
	h := handler.NewChatHandler(mockSvc)

	userID := uuid.New()
	messageID := uuid.New()
	roomID := uuid.New()

	mockSvc.On("DeleteMessage", mock.Anything, messageID, userID).Return(nil)

	c, rec := echotest.ContextConfig{
		PathValues: []echo.PathValue{
			{Name: "roomId", Value: roomID.String()},
			{Name: "messageId", Value: messageID.String()},
		},
	}.ToContextRecorder(t)
	c.Set(sharedauth.ClaimsContextKey, &sharedauth.Claims{RegisteredClaims: jwt.RegisteredClaims{Subject: userID.String()}})

	err := h.DeleteMessage(c)
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, http.StatusOK, rec.Code)

	var resp response.Response[any]
	err = json.Unmarshal(rec.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.True(t, resp.Success)

	mockSvc.AssertExpectations(t)
}

func TestChatHandler_DeleteMessage_InvalidMessageID(t *testing.T) {
	mockSvc := new(MockChatService)
	h := handler.NewChatHandler(mockSvc)

	rec := echotest.ContextConfig{
		PathValues: []echo.PathValue{
			{Name: "roomId", Value: uuid.New().String()},
			{Name: "messageId", Value: "not-a-uuid"},
		},
	}.ServeWithHandler(t, h.DeleteMessage)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	mockSvc.AssertNotCalled(t, "DeleteMessage", mock.Anything, mock.Anything, mock.Anything)
}

func strPtr(s string) *string {
	return &s
}
