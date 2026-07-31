package handler_test

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/Sephy314/chinwag/backend/services/chat/query/handler"
	"github.com/Sephy314/chinwag/backend/services/chat/query/structs"
	"github.com/Sephy314/chinwag/backend/services/chat/query/shared/response"
	sharedauth "github.com/Sephy314/chinwag/backend/shared/auth"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/labstack/echo/v5"
	"github.com/labstack/echo/v5/echotest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type MockQueryService struct {
	mock.Mock
}

func (m *MockQueryService) GetMessage(ctx context.Context, messageId uuid.UUID, userId uuid.UUID) (*structs.MessageResponse, error) {
	args := m.Called(ctx, messageId, userId)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*structs.MessageResponse), args.Error(1)
}

func (m *MockQueryService) ListMessages(ctx context.Context, req structs.ListMessagesRequest) ([]structs.MessageResponse, *structs.CursorMeta, error) {
	args := m.Called(ctx, req)
	if args.Get(0) == nil {
		return nil, args.Get(1).(*structs.CursorMeta), args.Error(2)
	}
	return args.Get(0).([]structs.MessageResponse), args.Get(1).(*structs.CursorMeta), args.Error(2)
}

func TestQueryHandler_Health(t *testing.T) {
	mockSvc := new(MockQueryService)
	h := handler.NewQueryHandler(mockSvc)

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

func TestQueryHandler_GetMessage_Success(t *testing.T) {
	mockSvc := new(MockQueryService)
	h := handler.NewQueryHandler(mockSvc)

	messageID := uuid.New()
	roomID := uuid.New()
	userID := uuid.New()
	now := time.Now()

	expected := &structs.MessageResponse{
		Id:          messageID.String(),
		RoomId:      roomID.String(),
		AuthorId:    uuid.New().String(),
		AuthorName:  "testuser",
		MessageType: 0,
		Content:     "Hello",
		CreatedAt:   now,
	}

	mockSvc.On("GetMessage", mock.Anything, messageID, userID).Return(expected, nil)

	c, rec := echotest.ContextConfig{
		PathValues: []echo.PathValue{
			{Name: "roomId", Value: roomID.String()},
			{Name: "messageId", Value: messageID.String()},
		},
	}.ToContextRecorder(t)
	c.Set(sharedauth.ClaimsContextKey, &sharedauth.Claims{
		RegisteredClaims: jwt.RegisteredClaims{Subject: userID.String()},
	})

	err := h.GetMessage(c)
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, http.StatusOK, rec.Code)
	var resp response.Response[structs.MessageResponse]
	err = json.Unmarshal(rec.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.True(t, resp.Success)
	assert.Equal(t, "Hello", resp.Data.Content)
	mockSvc.AssertExpectations(t)
}

func TestQueryHandler_GetMessage_InvalidMessageID(t *testing.T) {
	mockSvc := new(MockQueryService)
	h := handler.NewQueryHandler(mockSvc)

	rec := echotest.ContextConfig{
		PathValues: []echo.PathValue{
			{Name: "roomId", Value: uuid.New().String()},
			{Name: "messageId", Value: "not-a-uuid"},
		},
	}.ServeWithHandler(t, h.GetMessage)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	mockSvc.AssertNotCalled(t, "GetMessage", mock.Anything, mock.Anything, mock.Anything)
}

func TestQueryHandler_ListMessages_Success(t *testing.T) {
	mockSvc := new(MockQueryService)
	h := handler.NewQueryHandler(mockSvc)

	roomID := uuid.New()
	userID := uuid.New()
	now := time.Now()

	msgs := []structs.MessageResponse{
		{
			Id:          uuid.New().String(),
			RoomId:      roomID.String(),
			AuthorId:    uuid.New().String(),
			AuthorName:  "user1",
			MessageType: 0,
			Content:     "First",
			CreatedAt:   now,
		},
	}

	req := structs.ListMessagesRequest{
		RoomID: roomID.String(),
		Cursor: "",
		Limit:  50,
	}
	mockSvc.On("ListMessages", mock.Anything, req).Return(msgs, (*structs.CursorMeta)(nil), nil)

	c, rec := echotest.ContextConfig{
		PathValues: []echo.PathValue{
			{Name: "roomId", Value: roomID.String()},
		},
	}.ToContextRecorder(t)
	c.Set(sharedauth.ClaimsContextKey, &sharedauth.Claims{
		RegisteredClaims: jwt.RegisteredClaims{Subject: userID.String()},
	})

	err := h.ListMessages(c)
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, http.StatusOK, rec.Code)
	var resp response.Response[[]structs.MessageResponse]
	err = json.Unmarshal(rec.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.True(t, resp.Success)
	assert.Len(t, resp.Data, 1)
	assert.Equal(t, "First", resp.Data[0].Content)
	mockSvc.AssertExpectations(t)
}

func TestQueryHandler_ListMessages_WithCursor(t *testing.T) {
	mockSvc := new(MockQueryService)
	h := handler.NewQueryHandler(mockSvc)

	roomID := uuid.New()
	userID := uuid.New()
	now := time.Now()
	cursor := "eyJjcmVhdGVkX2F0IjoiMjAyNi0wNy0yM1QxOTo1OToxMloiLCJpZCI6IjU1MGU4NDAwLWUyOWItNDFkNC1hNzE2LTQ0NjY1NTQ0MDAwMCJ9"

	msgs := []structs.MessageResponse{
		{
			Id:          uuid.New().String(),
			RoomId:      roomID.String(),
			AuthorId:    uuid.New().String(),
			AuthorName:  "user1",
			MessageType: 0,
			Content:     "Next page",
			CreatedAt:   now,
		},
	}

	meta := &structs.CursorMeta{
		NextCursor: cursor,
		HasMore:    false,
	}

	req := structs.ListMessagesRequest{
		RoomID: roomID.String(),
		Cursor: cursor,
		Limit:  50,
	}
	mockSvc.On("ListMessages", mock.Anything, req).Return(msgs, meta, nil)

	c, rec := echotest.ContextConfig{
		PathValues: []echo.PathValue{
			{Name: "roomId", Value: roomID.String()},
		},
		QueryValues: map[string][]string{
			"cursor": {cursor},
		},
	}.ToContextRecorder(t)
	c.Set(sharedauth.ClaimsContextKey, &sharedauth.Claims{
		RegisteredClaims: jwt.RegisteredClaims{Subject: userID.String()},
	})

	err := h.ListMessages(c)
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, http.StatusOK, rec.Code)
	var resp response.Response[[]structs.MessageResponse]
	err = json.Unmarshal(rec.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.True(t, resp.Success)
	assert.Len(t, resp.Data, 1)
	mockSvc.AssertExpectations(t)
}

func TestQueryHandler_ListMessages_Unauthorized(t *testing.T) {
	h := handler.NewQueryHandler(nil)

	rec := echotest.ContextConfig{
		PathValues: []echo.PathValue{
			{Name: "roomId", Value: uuid.New().String()},
		},
	}.ServeWithHandler(t, h.ListMessages)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}
