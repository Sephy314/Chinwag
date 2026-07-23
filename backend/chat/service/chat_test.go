package service

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/Sephy314/chinwag/chat/domain"
	"github.com/Sephy314/chinwag/chat/structs"
	"github.com/Sephy314/chinwag/conn/bridge"
	"github.com/Sephy314/chinwag/shared/errs"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type MockChatRepo struct {
	mock.Mock
}

func (m *MockChatRepo) CreateMessage(ctx context.Context, msg domain.ChatMessage) error {
	args := m.Called(ctx, msg)
	return args.Error(0)
}

func (m *MockChatRepo) GetMessageById(ctx context.Context, id uuid.UUID) (domain.ChatMessage, error) {
	args := m.Called(ctx, id)
	return args.Get(0).(domain.ChatMessage), args.Error(1)
}

func (m *MockChatRepo) ListMessagesByRoomId(ctx context.Context, roomId uuid.UUID, cursorStr string, limit int) ([]domain.ChatMessage, *structs.CursorMeta, error) {
	args := m.Called(ctx, roomId, cursorStr, limit)
	if args.Get(0) == nil {
		return nil, args.Get(1).(*structs.CursorMeta), args.Error(2)
	}
	return args.Get(0).([]domain.ChatMessage), args.Get(1).(*structs.CursorMeta), args.Error(2)
}

func (m *MockChatRepo) UpdateMessage(ctx context.Context, msg domain.ChatMessage) error {
	args := m.Called(ctx, msg)
	return args.Error(0)
}

func (m *MockChatRepo) DeleteMessage(ctx context.Context, id uuid.UUID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

type MockUserProvider struct {
	mock.Mock
}

func (m *MockUserProvider) GetUser(ctx context.Context, id string) (*bridge.UserInfo, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*bridge.UserInfo), args.Error(1)
}

type MockMemberProvider struct {
	mock.Mock
}

func (m *MockMemberProvider) GetRoomsByUserId(ctx context.Context, userId string) ([]bridge.RoomInfo, error) {
	args := m.Called(ctx, userId)
	return args.Get(0).([]bridge.RoomInfo), args.Error(1)
}

func (m *MockMemberProvider) GetMembersByRoomId(ctx context.Context, roomId string) ([]bridge.RoomMemberInfo, error) {
	args := m.Called(ctx, roomId)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]bridge.RoomMemberInfo), args.Error(1)
}

func TestCreateMessage_Success(t *testing.T) {
	mockRepo := new(MockChatRepo)
	mockUser := new(MockUserProvider)
	mockMember := new(MockMemberProvider)

	authorId := uuid.New()
	roomId := uuid.New()
	ctx := context.WithValue(context.Background(), "authorId", authorId)

	req := structs.CreateMessageRequest{
		MessageType: domain.MessageTypeTEXT,
		Content:     "Hello, world!",
	}

	mockMember.On("GetMembersByRoomId", ctx, roomId.String()).Return([]bridge.RoomMemberInfo{
		{UserId: authorId.String(), RoomId: roomId.String()},
	}, nil)

	mockUser.On("GetUser", ctx, authorId.String()).Return(&bridge.UserInfo{
		Id:   authorId.String(),
		Name: "testuser",
	}, nil)

	mockRepo.On("CreateMessage", ctx, mock.MatchedBy(func(msg domain.ChatMessage) bool {
		return msg.Content == "Hello, world!" && msg.RoomId == roomId && msg.AuthorId == authorId
	})).Return(nil)

	svc := NewChatService(mockRepo, nil, mockUser, mockMember, nil)
	result, err := svc.CreateMessage(ctx, roomId, req)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, "Hello, world!", result.Content)
	assert.Equal(t, "testuser", result.AuthorName)
	assert.Equal(t, authorId.String(), result.AuthorId)
	mockRepo.AssertExpectations(t)
	mockUser.AssertExpectations(t)
	mockMember.AssertExpectations(t)
}

func TestCreateMessage_NotMember(t *testing.T) {
	mockRepo := new(MockChatRepo)
	mockUser := new(MockUserProvider)
	mockMember := new(MockMemberProvider)

	authorId := uuid.New()
	roomId := uuid.New()
	ctx := context.WithValue(context.Background(), "authorId", authorId)

	req := structs.CreateMessageRequest{
		MessageType: domain.MessageTypeTEXT,
		Content:     "Hello",
	}

	mockMember.On("GetMembersByRoomId", ctx, roomId.String()).Return([]bridge.RoomMemberInfo{}, nil)

	svc := NewChatService(mockRepo, nil, mockUser, mockMember, nil)
	result, err := svc.CreateMessage(ctx, roomId, req)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Equal(t, http.StatusForbidden, err.(*errs.AppError).Status)
	mockRepo.AssertNotCalled(t, "CreateMessage", mock.Anything, mock.Anything)
	mockMember.AssertExpectations(t)
}

func TestCreateMessage_MemberProviderError(t *testing.T) {
	mockRepo := new(MockChatRepo)
	mockUser := new(MockUserProvider)
	mockMember := new(MockMemberProvider)

	authorId := uuid.New()
	roomId := uuid.New()
	ctx := context.WithValue(context.Background(), "authorId", authorId)

	req := structs.CreateMessageRequest{
		MessageType: domain.MessageTypeTEXT,
		Content:     "Hello",
	}

	mockMember.On("GetMembersByRoomId", ctx, roomId.String()).Return(nil, errors.New("member service error"))

	svc := NewChatService(mockRepo, nil, mockUser, mockMember, nil)
	result, err := svc.CreateMessage(ctx, roomId, req)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Equal(t, "member service error", err.Error())
	mockRepo.AssertNotCalled(t, "CreateMessage", mock.Anything, mock.Anything)
	mockMember.AssertExpectations(t)
}

func TestCreateMessage_UserProviderError(t *testing.T) {
	mockRepo := new(MockChatRepo)
	mockUser := new(MockUserProvider)
	mockMember := new(MockMemberProvider)

	authorId := uuid.New()
	roomId := uuid.New()
	ctx := context.WithValue(context.Background(), "authorId", authorId)

	req := structs.CreateMessageRequest{
		MessageType: domain.MessageTypeTEXT,
		Content:     "Hello",
	}

	mockMember.On("GetMembersByRoomId", ctx, roomId.String()).Return([]bridge.RoomMemberInfo{
		{UserId: authorId.String(), RoomId: roomId.String()},
	}, nil)

	mockUser.On("GetUser", ctx, authorId.String()).Return(nil, errors.New("user service error"))

	svc := NewChatService(mockRepo, nil, mockUser, mockMember, nil)
	result, err := svc.CreateMessage(ctx, roomId, req)

	assert.Error(t, err)
	assert.Nil(t, result)
	mockRepo.AssertNotCalled(t, "CreateMessage", mock.Anything, mock.Anything)
	mockMember.AssertExpectations(t)
	mockUser.AssertExpectations(t)
}

func TestCreateMessage_RepoError(t *testing.T) {
	mockRepo := new(MockChatRepo)
	mockUser := new(MockUserProvider)
	mockMember := new(MockMemberProvider)

	authorId := uuid.New()
	roomId := uuid.New()
	ctx := context.WithValue(context.Background(), "authorId", authorId)

	req := structs.CreateMessageRequest{
		MessageType: domain.MessageTypeTEXT,
		Content:     "Hello",
	}

	mockMember.On("GetMembersByRoomId", ctx, roomId.String()).Return([]bridge.RoomMemberInfo{
		{UserId: authorId.String(), RoomId: roomId.String()},
	}, nil)

	mockUser.On("GetUser", ctx, authorId.String()).Return(&bridge.UserInfo{
		Id:   authorId.String(),
		Name: "testuser",
	}, nil)

	mockRepo.On("CreateMessage", ctx, mock.Anything).Return(errors.New("db error"))

	svc := NewChatService(mockRepo, nil, mockUser, mockMember, nil)
	result, err := svc.CreateMessage(ctx, roomId, req)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Equal(t, "db error", err.Error())
	mockRepo.AssertExpectations(t)
}

func TestGetMessage_Success(t *testing.T) {
	mockRepo := new(MockChatRepo)
	mockUser := new(MockUserProvider)
	mockMember := new(MockMemberProvider)

	authorId := uuid.New()
	messageId := uuid.New()
	roomId := uuid.New()
	now := time.Now()
	ctx := context.Background()

	msg := domain.ChatMessage{
		Id:          messageId,
		RoomId:      roomId,
		AuthorId:    authorId,
		MessageType: domain.MessageTypeTEXT,
		Content:     "Hello",
		CreatedAt:   now,
	}

	mockRepo.On("GetMessageById", ctx, messageId).Return(msg, nil)
	mockUser.On("GetUser", ctx, authorId.String()).Return(&bridge.UserInfo{
		Id:   authorId.String(),
		Name: "testuser",
	}, nil)

	svc := NewChatService(mockRepo, nil, mockUser, mockMember, nil)
	result, err := svc.GetMessage(ctx, messageId)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, messageId.String(), result.Id)
	assert.Equal(t, "Hello", result.Content)
	assert.Equal(t, "testuser", result.AuthorName)
	mockRepo.AssertExpectations(t)
	mockUser.AssertExpectations(t)
}

func TestGetMessage_NotFound(t *testing.T) {
	mockRepo := new(MockChatRepo)
	mockUser := new(MockUserProvider)
	mockMember := new(MockMemberProvider)

	messageId := uuid.New()
	ctx := context.Background()

	mockRepo.On("GetMessageById", ctx, messageId).Return(domain.ChatMessage{}, errs.ErrNotFound)

	svc := NewChatService(mockRepo, nil, mockUser, mockMember, nil)
	result, err := svc.GetMessage(ctx, messageId)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Equal(t, errs.ErrNotFound, err)
	mockRepo.AssertExpectations(t)
}

func TestUpdateMessage_Success(t *testing.T) {
	mockRepo := new(MockChatRepo)
	mockUser := new(MockUserProvider)
	mockMember := new(MockMemberProvider)

	authorId := uuid.New()
	messageId := uuid.New()
	roomId := uuid.New()
	now := time.Now()
	ctx := context.Background()

	existing := domain.ChatMessage{
		Id:          messageId,
		RoomId:      roomId,
		AuthorId:    authorId,
		MessageType: domain.MessageTypeTEXT,
		Content:     "Old content",
		CreatedAt:   now,
	}

	newContent := "Updated content"
	req := structs.UpdateMessageRequest{Content: &newContent}

	updated := existing
	updated.Content = newContent
	updated.UpdatedAt = new(now.Add(time.Minute))

	mockRepo.On("GetMessageById", ctx, messageId).Return(existing, nil).Once()
	mockRepo.On("UpdateMessage", ctx, mock.MatchedBy(func(msg domain.ChatMessage) bool {
		return msg.Content == "Updated content" && msg.Id == messageId
	})).Return(nil)
	mockRepo.On("GetMessageById", ctx, messageId).Return(updated, nil).Once()

	mockUser.On("GetUser", ctx, authorId.String()).Return(&bridge.UserInfo{
		Id:   authorId.String(),
		Name: "testuser",
	}, nil)

	svc := NewChatService(mockRepo, nil, mockUser, mockMember, nil)
	result, err := svc.UpdateMessage(ctx, messageId, authorId, req)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, "Updated content", result.Content)
	mockRepo.AssertExpectations(t)
	mockUser.AssertExpectations(t)
}

func TestUpdateMessage_NotAuthor(t *testing.T) {
	mockRepo := new(MockChatRepo)
	mockUser := new(MockUserProvider)
	mockMember := new(MockMemberProvider)

	authorId := uuid.New()
	otherUserId := uuid.New()
	messageId := uuid.New()
	roomId := uuid.New()
	ctx := context.Background()

	existing := domain.ChatMessage{
		Id:       messageId,
		RoomId:   roomId,
		AuthorId: authorId,
		Content:  "Old",
	}

	mockRepo.On("GetMessageById", ctx, messageId).Return(existing, nil)

	svc := NewChatService(mockRepo, nil, mockUser, mockMember, nil)
	result, err := svc.UpdateMessage(ctx, messageId, otherUserId, structs.UpdateMessageRequest{})

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Equal(t, http.StatusForbidden, err.(*errs.AppError).Status)
	mockRepo.AssertExpectations(t)
}

func TestUpdateMessage_NotFound(t *testing.T) {
	mockRepo := new(MockChatRepo)
	mockUser := new(MockUserProvider)
	mockMember := new(MockMemberProvider)

	authorId := uuid.New()
	messageId := uuid.New()
	ctx := context.Background()

	mockRepo.On("GetMessageById", ctx, messageId).Return(domain.ChatMessage{}, errs.ErrNotFound)

	svc := NewChatService(mockRepo, nil, mockUser, mockMember, nil)
	result, err := svc.UpdateMessage(ctx, messageId, authorId, structs.UpdateMessageRequest{})

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Equal(t, errs.ErrNotFound, err)
	mockRepo.AssertExpectations(t)
}

func TestDeleteMessage_Success(t *testing.T) {
	mockRepo := new(MockChatRepo)
	mockUser := new(MockUserProvider)
	mockMember := new(MockMemberProvider)

	authorId := uuid.New()
	messageId := uuid.New()
	roomId := uuid.New()
	ctx := context.Background()

	existing := domain.ChatMessage{
		Id:       messageId,
		RoomId:   roomId,
		AuthorId: authorId,
		Content:  "To delete",
	}

	mockRepo.On("GetMessageById", ctx, messageId).Return(existing, nil)
	mockRepo.On("DeleteMessage", ctx, messageId).Return(nil)

	svc := NewChatService(mockRepo, nil, mockUser, mockMember, nil)
	err := svc.DeleteMessage(ctx, messageId, authorId)

	assert.NoError(t, err)
	mockRepo.AssertExpectations(t)
}

func TestDeleteMessage_NotAuthor(t *testing.T) {
	mockRepo := new(MockChatRepo)
	mockUser := new(MockUserProvider)
	mockMember := new(MockMemberProvider)

	authorId := uuid.New()
	otherUserId := uuid.New()
	messageId := uuid.New()
	roomId := uuid.New()
	ctx := context.Background()

	existing := domain.ChatMessage{
		Id:       messageId,
		RoomId:   roomId,
		AuthorId: authorId,
		Content:  "To delete",
	}

	mockRepo.On("GetMessageById", ctx, messageId).Return(existing, nil)

	svc := NewChatService(mockRepo, nil, mockUser, mockMember, nil)
	err := svc.DeleteMessage(ctx, messageId, otherUserId)

	assert.Error(t, err)
	assert.Equal(t, http.StatusForbidden, err.(*errs.AppError).Status)
	mockRepo.AssertExpectations(t)
}

func TestDeleteMessage_NotFound(t *testing.T) {
	mockRepo := new(MockChatRepo)
	mockUser := new(MockUserProvider)
	mockMember := new(MockMemberProvider)

	authorId := uuid.New()
	messageId := uuid.New()
	ctx := context.Background()

	mockRepo.On("GetMessageById", ctx, messageId).Return(domain.ChatMessage{}, errs.ErrNotFound)

	svc := NewChatService(mockRepo, nil, mockUser, mockMember, nil)
	err := svc.DeleteMessage(ctx, messageId, authorId)

	assert.Error(t, err)
	assert.Equal(t, errs.ErrNotFound, err)
	mockRepo.AssertExpectations(t)
}

func TestListMessages_Success(t *testing.T) {
	mockRepo := new(MockChatRepo)
	mockUser := new(MockUserProvider)
	mockMember := new(MockMemberProvider)

	roomId := uuid.New()
	authorId := uuid.New()
	now := time.Now()
	ctx := context.Background()

	msgs := []domain.ChatMessage{
		{
			Id:          uuid.New(),
			RoomId:      roomId,
			AuthorId:    authorId,
			MessageType: domain.MessageTypeTEXT,
			Content:     "First",
			CreatedAt:   now,
		},
		{
			Id:          uuid.New(),
			RoomId:      roomId,
			AuthorId:    authorId,
			MessageType: domain.MessageTypeTEXT,
			Content:     "Second",
			CreatedAt:   now.Add(-time.Minute),
		},
	}

	mockRepo.On("ListMessagesByRoomId", ctx, roomId, "", 50).Return(msgs, (*structs.CursorMeta)(nil), nil)
	mockUser.On("GetUser", ctx, authorId.String()).Return(&bridge.UserInfo{
		Id:   authorId.String(),
		Name: "testuser",
	}, nil)

	req := structs.ListMessagesRequest{
		RoomID: roomId.String(),
		Cursor: "",
		Limit:  50,
	}

	svc := NewChatService(mockRepo, nil, mockUser, mockMember, nil)
	result, meta, err := svc.ListMessages(ctx, req)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Nil(t, meta)
	assert.Equal(t, 2, len(result))
	assert.Equal(t, "testuser", result[0].AuthorName)
	mockRepo.AssertExpectations(t)
}

func TestListMessages_Empty(t *testing.T) {
	mockRepo := new(MockChatRepo)
	mockUser := new(MockUserProvider)
	mockMember := new(MockMemberProvider)

	roomId := uuid.New()
	ctx := context.Background()

	mockRepo.On("ListMessagesByRoomId", ctx, roomId, "", 50).Return([]domain.ChatMessage{}, (*structs.CursorMeta)(nil), nil)

	req := structs.ListMessagesRequest{
		RoomID: roomId.String(),
		Cursor: "",
		Limit:  50,
	}

	svc := NewChatService(mockRepo, nil, mockUser, mockMember, nil)
	result, meta, err := svc.ListMessages(ctx, req)

	assert.NoError(t, err)
	assert.Equal(t, 0, len(result))
	assert.Nil(t, meta)
	mockRepo.AssertExpectations(t)
}

func TestListMessages_InvalidRoomID(t *testing.T) {
	svc := NewChatService(nil, nil, nil, nil, nil)

	req := structs.ListMessagesRequest{
		RoomID: "not-a-uuid",
	}

	result, meta, err := svc.ListMessages(context.Background(), req)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Nil(t, meta)
}

func TestCreateMessage_NonExistentRoom(t *testing.T) {
	mockRepo := new(MockChatRepo)
	mockUser := new(MockUserProvider)
	mockMember := new(MockMemberProvider)

	authorId := uuid.New()
	roomId := uuid.New()
	ctx := context.WithValue(context.Background(), "authorId", authorId)

	req := structs.CreateMessageRequest{
		MessageType: domain.MessageTypeTEXT,
		Content:     "Hello",
	}

	mockMember.On("GetMembersByRoomId", ctx, roomId.String()).Return(nil, errs.ErrNotFound)

	svc := NewChatService(mockRepo, nil, mockUser, mockMember, nil)
	result, err := svc.CreateMessage(ctx, roomId, req)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Equal(t, errs.ErrNotFound, err)
	mockRepo.AssertNotCalled(t, "CreateMessage", mock.Anything, mock.Anything)
	mockMember.AssertExpectations(t)
}
