package service

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/Sephy314/chinwag/backend/services/chat/query/domain"
	"github.com/Sephy314/chinwag/backend/services/chat/query/structs"
	"github.com/Sephy314/chinwag/backend/services/chat/query/shared/errs"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type MockProjectionRepo struct {
	mock.Mock
}

func (m *MockProjectionRepo) Upsert(ctx context.Context, msg domain.MessageProjection) error {
	args := m.Called(ctx, msg)
	return args.Error(0)
}

func (m *MockProjectionRepo) GetById(ctx context.Context, id uuid.UUID) (domain.MessageProjection, error) {
	args := m.Called(ctx, id)
	return args.Get(0).(domain.MessageProjection), args.Error(1)
}

func (m *MockProjectionRepo) ListByRoomId(ctx context.Context, roomId uuid.UUID, cursorStr string, limit int) ([]domain.MessageProjection, *structs.CursorMeta, error) {
	args := m.Called(ctx, roomId, cursorStr, limit)
	if args.Get(0) == nil {
		return nil, args.Get(1).(*structs.CursorMeta), args.Error(2)
	}
	return args.Get(0).([]domain.MessageProjection), args.Get(1).(*structs.CursorMeta), args.Error(2)
}

func (m *MockProjectionRepo) UpdateContent(ctx context.Context, id uuid.UUID, content string, updatedAt time.Time) error {
	args := m.Called(ctx, id, content, updatedAt)
	return args.Error(0)
}

func (m *MockProjectionRepo) SoftDelete(ctx context.Context, id uuid.UUID, deletedAt time.Time) error {
	args := m.Called(ctx, id, deletedAt)
	return args.Error(0)
}

type MockMemberProvider struct {
	mock.Mock
}

func (m *MockMemberProvider) GetMembersByRoomId(ctx context.Context, roomId string) ([]RoomMemberInfo, error) {
	args := m.Called(ctx, roomId)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]RoomMemberInfo), args.Error(1)
}

func TestGetMessage_Success(t *testing.T) {
	mockRepo := new(MockProjectionRepo)
	mockMember := new(MockMemberProvider)

	messageId := uuid.New()
	roomId := uuid.New()
	authorId := uuid.New()
	userId := authorId
	now := time.Now()

	msg := domain.MessageProjection{
		Id:          messageId,
		RoomId:      roomId,
		AuthorId:    authorId,
		AuthorName:  "testuser",
		MessageType: 0,
		Content:     "Hello",
		CreatedAt:   now,
	}

	mockRepo.On("GetById", mock.Anything, messageId).Return(msg, nil)
	mockMember.On("GetMembersByRoomId", mock.Anything, roomId.String()).Return([]RoomMemberInfo{
		{UserId: userId.String(), RoomId: roomId.String()},
	}, nil)

	svc := NewQueryService(mockRepo, mockMember)
	result, err := svc.GetMessage(context.Background(), messageId, userId)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, "Hello", result.Content)
	assert.Equal(t, "testuser", result.AuthorName)
	mockRepo.AssertExpectations(t)
	mockMember.AssertExpectations(t)
}

func TestGetMessage_NotFound(t *testing.T) {
	mockRepo := new(MockProjectionRepo)
	mockMember := new(MockMemberProvider)

	messageId := uuid.New()

	mockRepo.On("GetById", mock.Anything, messageId).Return(domain.MessageProjection{}, errs.ErrNotFound)

	svc := NewQueryService(mockRepo, mockMember)
	result, err := svc.GetMessage(context.Background(), messageId, uuid.New())

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Equal(t, errs.ErrNotFound, err)
	mockRepo.AssertExpectations(t)
}

func TestGetMessage_NotMember(t *testing.T) {
	mockRepo := new(MockProjectionRepo)
	mockMember := new(MockMemberProvider)

	messageId := uuid.New()
	roomId := uuid.New()
	userId := uuid.New()

	msg := domain.MessageProjection{
		Id:     messageId,
		RoomId: roomId,
	}

	mockRepo.On("GetById", mock.Anything, messageId).Return(msg, nil)
	mockMember.On("GetMembersByRoomId", mock.Anything, roomId.String()).Return([]RoomMemberInfo{}, nil)

	svc := NewQueryService(mockRepo, mockMember)
	result, err := svc.GetMessage(context.Background(), messageId, userId)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Equal(t, http.StatusForbidden, err.(*errs.AppError).Status)
	mockRepo.AssertExpectations(t)
	mockMember.AssertExpectations(t)
}

func TestGetMessage_MemberProviderError(t *testing.T) {
	mockRepo := new(MockProjectionRepo)
	mockMember := new(MockMemberProvider)

	messageId := uuid.New()
	roomId := uuid.New()

	msg := domain.MessageProjection{
		Id:     messageId,
		RoomId: roomId,
	}

	mockRepo.On("GetById", mock.Anything, messageId).Return(msg, nil)
	mockMember.On("GetMembersByRoomId", mock.Anything, roomId.String()).Return(nil, errors.New("member service error"))

	svc := NewQueryService(mockRepo, mockMember)
	result, err := svc.GetMessage(context.Background(), messageId, uuid.New())

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Equal(t, "member service error", err.Error())
}

func TestListMessages_Success(t *testing.T) {
	mockRepo := new(MockProjectionRepo)
	mockMember := new(MockMemberProvider)

	roomId := uuid.New()
	userId := uuid.New()
	now := time.Now()

	msgs := []domain.MessageProjection{
		{Id: uuid.New(), RoomId: roomId, AuthorId: userId, AuthorName: "user1", Content: "First", CreatedAt: now},
	}

	mockMember.On("GetMembersByRoomId", mock.Anything, roomId.String()).Return([]RoomMemberInfo{
		{UserId: userId.String(), RoomId: roomId.String()},
	}, nil)
	mockRepo.On("ListByRoomId", mock.Anything, roomId, "", 50).Return(msgs, (*structs.CursorMeta)(nil), nil)

	ctx := context.WithValue(context.Background(), "userId", userId)
	svc := NewQueryService(mockRepo, mockMember)

	req := structs.ListMessagesRequest{RoomID: roomId.String(), Cursor: "", Limit: 50}
	result, meta, err := svc.ListMessages(ctx, req)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Len(t, result, 1)
	assert.Nil(t, meta)
	mockRepo.AssertExpectations(t)
	mockMember.AssertExpectations(t)
}

func TestListMessages_Empty(t *testing.T) {
	mockRepo := new(MockProjectionRepo)
	mockMember := new(MockMemberProvider)

	roomId := uuid.New()
	userId := uuid.New()

	mockMember.On("GetMembersByRoomId", mock.Anything, roomId.String()).Return([]RoomMemberInfo{
		{UserId: userId.String(), RoomId: roomId.String()},
	}, nil)
	mockRepo.On("ListByRoomId", mock.Anything, roomId, "", 50).Return([]domain.MessageProjection{}, (*structs.CursorMeta)(nil), nil)

	ctx := context.WithValue(context.Background(), "userId", userId)
	svc := NewQueryService(mockRepo, mockMember)

	req := structs.ListMessagesRequest{RoomID: roomId.String(), Cursor: "", Limit: 50}
	result, meta, err := svc.ListMessages(ctx, req)

	assert.NoError(t, err)
	assert.Empty(t, result)
	assert.Nil(t, meta)
}

func TestListMessages_NotMember(t *testing.T) {
	mockRepo := new(MockProjectionRepo)
	mockMember := new(MockMemberProvider)

	roomId := uuid.New()
	userId := uuid.New()

	mockMember.On("GetMembersByRoomId", mock.Anything, roomId.String()).Return([]RoomMemberInfo{}, nil)

	ctx := context.WithValue(context.Background(), "userId", userId)
	svc := NewQueryService(mockRepo, mockMember)

	req := structs.ListMessagesRequest{RoomID: roomId.String(), Cursor: "", Limit: 50}
	result, meta, err := svc.ListMessages(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Nil(t, meta)
	assert.Equal(t, http.StatusForbidden, err.(*errs.AppError).Status)
}

func TestListMessages_InvalidRoomID(t *testing.T) {
	svc := NewQueryService(nil, nil)

	req := structs.ListMessagesRequest{RoomID: "not-a-uuid"}
	result, meta, err := svc.ListMessages(context.Background(), req)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Nil(t, meta)
}

func TestListMessages_MemberProviderError(t *testing.T) {
	mockRepo := new(MockProjectionRepo)
	mockMember := new(MockMemberProvider)

	roomId := uuid.New()
	userId := uuid.New()

	mockMember.On("GetMembersByRoomId", mock.Anything, roomId.String()).Return(nil, errors.New("member service error"))

	ctx := context.WithValue(context.Background(), "userId", userId)
	svc := NewQueryService(mockRepo, mockMember)

	req := structs.ListMessagesRequest{RoomID: roomId.String(), Cursor: "", Limit: 50}
	result, meta, err := svc.ListMessages(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Nil(t, meta)
}
