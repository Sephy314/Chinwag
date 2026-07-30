package service

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/Sephy314/chinwag/backend/services/chat/command/domain"
	"github.com/Sephy314/chinwag/backend/services/chat/command/repo"
	"github.com/Sephy314/chinwag/backend/services/chat/command/structs"
	"github.com/Sephy314/chinwag/backend/services/chat/command/shared/errs"
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

type MockOutboxRepo struct {
	mock.Mock
}

func (m *MockOutboxRepo) Insert(ctx context.Context, event repo.OutboxEvent) error {
	args := m.Called(ctx, event)
	return args.Error(0)
}

func (m *MockOutboxRepo) PollPending(ctx context.Context, batchSize int) ([]repo.OutboxEvent, error) {
	args := m.Called(ctx, batchSize)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]repo.OutboxEvent), args.Error(1)
}

func (m *MockOutboxRepo) MarkPublished(ctx context.Context, id uuid.UUID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockOutboxRepo) IncrementRetry(ctx context.Context, id uuid.UUID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

type testTransaction struct {
	chatRepo   repo.ChatRepoInterface
	outboxRepo repo.OutboxRepoInterface
}

func (t *testTransaction) ChatRepo() repo.ChatRepoInterface   { return t.chatRepo }
func (t *testTransaction) OutboxRepo() repo.OutboxRepoInterface { return t.outboxRepo }

type testUnitOfWork struct {
	chatRepo   repo.ChatRepoInterface
	outboxRepo repo.OutboxRepoInterface
}

func (u *testUnitOfWork) Do(ctx context.Context, fn func(ctx context.Context, tx repo.Transaction) error) error {
	return fn(ctx, &testTransaction{chatRepo: u.chatRepo, outboxRepo: u.outboxRepo})
}

type MockUserProvider struct {
	mock.Mock
}

func (m *MockUserProvider) GetUser(ctx context.Context, id string) (*UserInfo, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*UserInfo), args.Error(1)
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

func (m *MockMemberProvider) GetRoomById(ctx context.Context, roomId string) (*RoomInfo, error) {
	args := m.Called(ctx, roomId)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*RoomInfo), args.Error(1)
}

func makeUow(chatRepo repo.ChatRepoInterface, outboxRepo repo.OutboxRepoInterface) repo.UnitOfWork {
	return &testUnitOfWork{chatRepo: chatRepo, outboxRepo: outboxRepo}
}

func TestCreateMessage_Success(t *testing.T) {
	mockRepo := new(MockChatRepo)
	mockOutbox := new(MockOutboxRepo)
	mockUser := new(MockUserProvider)
	mockMember := new(MockMemberProvider)

	authorId := uuid.New()
	roomId := uuid.New()
	ctx := context.WithValue(context.Background(), "authorId", authorId)

	req := structs.CreateMessageRequest{
		MessageType: domain.MessageTypeTEXT,
		Content:     "Hello, world!",
	}

	mockMember.On("GetMembersByRoomId", ctx, roomId.String()).Return([]RoomMemberInfo{
		{UserId: authorId.String(), RoomId: roomId.String()},
	}, nil)

	mockMember.On("GetRoomById", ctx, roomId.String()).Return(&RoomInfo{
		Id: roomId.String(),
	}, nil)

	mockUser.On("GetUser", ctx, authorId.String()).Return(&UserInfo{
		Id:   authorId.String(),
		Name: "testuser",
	}, nil)

	mockRepo.On("CreateMessage", ctx, mock.MatchedBy(func(msg domain.ChatMessage) bool {
		return msg.Content == "Hello, world!" && msg.RoomId == roomId && msg.AuthorId == authorId
	})).Return(nil)

	mockOutbox.On("Insert", ctx, mock.MatchedBy(func(ev repo.OutboxEvent) bool {
		return ev.EventType == "message_created" && ev.RoomId == roomId && ev.Subject == "chat.room."+roomId.String()
	})).Return(nil)

	uow := makeUow(mockRepo, mockOutbox)
	svc := NewChatService(mockRepo, uow, mockUser, mockMember)
	result, err := svc.CreateMessage(ctx, roomId, req)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, "Hello, world!", result.Content)
	assert.Equal(t, "testuser", result.AuthorName)
	assert.Equal(t, authorId.String(), result.AuthorId)
	mockRepo.AssertExpectations(t)
	mockOutbox.AssertExpectations(t)
	mockUser.AssertExpectations(t)
	mockMember.AssertExpectations(t)
}

func TestCreateMessage_NotMember(t *testing.T) {
	mockRepo := new(MockChatRepo)
	mockOutbox := new(MockOutboxRepo)
	mockUser := new(MockUserProvider)
	mockMember := new(MockMemberProvider)

	authorId := uuid.New()
	roomId := uuid.New()
	ctx := context.WithValue(context.Background(), "authorId", authorId)

	req := structs.CreateMessageRequest{
		MessageType: domain.MessageTypeTEXT,
		Content:     "Hello",
	}

	mockMember.On("GetMembersByRoomId", ctx, roomId.String()).Return([]RoomMemberInfo{}, nil)

	mockMember.On("GetRoomById", ctx, roomId.String()).Return(&RoomInfo{
		Id: roomId.String(),
	}, nil)

	uow := makeUow(mockRepo, mockOutbox)
	svc := NewChatService(mockRepo, uow, mockUser, mockMember)
	result, err := svc.CreateMessage(ctx, roomId, req)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Equal(t, http.StatusForbidden, err.(*errs.AppError).Status)
	mockRepo.AssertNotCalled(t, "CreateMessage", mock.Anything, mock.Anything)
	mockMember.AssertExpectations(t)
}

func TestCreateMessage_MemberProviderError(t *testing.T) {
	mockRepo := new(MockChatRepo)
	mockOutbox := new(MockOutboxRepo)
	mockUser := new(MockUserProvider)
	mockMember := new(MockMemberProvider)

	authorId := uuid.New()
	roomId := uuid.New()
	ctx := context.WithValue(context.Background(), "authorId", authorId)

	req := structs.CreateMessageRequest{
		MessageType: domain.MessageTypeTEXT,
		Content:     "Hello",
	}

	mockMember.On("GetRoomById", ctx, roomId.String()).Return(nil, errors.New("member service error"))

	uow := makeUow(mockRepo, mockOutbox)
	svc := NewChatService(mockRepo, uow, mockUser, mockMember)
	result, err := svc.CreateMessage(ctx, roomId, req)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Equal(t, "member service error", err.Error())
	mockRepo.AssertNotCalled(t, "CreateMessage", mock.Anything, mock.Anything)
	mockMember.AssertExpectations(t)
}

func TestCreateMessage_UserProviderError(t *testing.T) {
	mockRepo := new(MockChatRepo)
	mockOutbox := new(MockOutboxRepo)
	mockUser := new(MockUserProvider)
	mockMember := new(MockMemberProvider)

	authorId := uuid.New()
	roomId := uuid.New()
	ctx := context.WithValue(context.Background(), "authorId", authorId)

	req := structs.CreateMessageRequest{
		MessageType: domain.MessageTypeTEXT,
		Content:     "Hello",
	}

	mockMember.On("GetRoomById", ctx, roomId.String()).Return(&RoomInfo{
		Id: roomId.String(),
	}, nil)

	mockMember.On("GetMembersByRoomId", ctx, roomId.String()).Return([]RoomMemberInfo{
		{UserId: authorId.String(), RoomId: roomId.String()},
	}, nil)

	mockUser.On("GetUser", ctx, authorId.String()).Return(nil, errors.New("user service error"))

	uow := makeUow(mockRepo, mockOutbox)
	svc := NewChatService(mockRepo, uow, mockUser, mockMember)
	result, err := svc.CreateMessage(ctx, roomId, req)

	assert.Error(t, err)
	assert.Nil(t, result)
	mockRepo.AssertNotCalled(t, "CreateMessage", mock.Anything, mock.Anything)
	mockMember.AssertExpectations(t)
	mockUser.AssertExpectations(t)
}

func TestCreateMessage_RepoError(t *testing.T) {
	mockRepo := new(MockChatRepo)
	mockOutbox := new(MockOutboxRepo)
	mockUser := new(MockUserProvider)
	mockMember := new(MockMemberProvider)

	authorId := uuid.New()
	roomId := uuid.New()
	ctx := context.WithValue(context.Background(), "authorId", authorId)

	req := structs.CreateMessageRequest{
		MessageType: domain.MessageTypeTEXT,
		Content:     "Hello",
	}

	mockMember.On("GetRoomById", ctx, roomId.String()).Return(&RoomInfo{
		Id: roomId.String(),
	}, nil)

	mockMember.On("GetMembersByRoomId", ctx, roomId.String()).Return([]RoomMemberInfo{
		{UserId: authorId.String(), RoomId: roomId.String()},
	}, nil)

	mockUser.On("GetUser", ctx, authorId.String()).Return(&UserInfo{
		Id:   authorId.String(),
		Name: "testuser",
	}, nil)

	mockRepo.On("CreateMessage", ctx, mock.Anything).Return(errors.New("db error"))

	uow := makeUow(mockRepo, mockOutbox)
	svc := NewChatService(mockRepo, uow, mockUser, mockMember)
	result, err := svc.CreateMessage(ctx, roomId, req)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Equal(t, "db error", err.Error())
	mockRepo.AssertExpectations(t)
}

func TestCreateMessage_OutboxError(t *testing.T) {
	mockRepo := new(MockChatRepo)
	mockOutbox := new(MockOutboxRepo)
	mockUser := new(MockUserProvider)
	mockMember := new(MockMemberProvider)

	authorId := uuid.New()
	roomId := uuid.New()
	ctx := context.WithValue(context.Background(), "authorId", authorId)

	req := structs.CreateMessageRequest{
		MessageType: domain.MessageTypeTEXT,
		Content:     "Hello, world!",
	}

	mockMember.On("GetRoomById", ctx, roomId.String()).Return(&RoomInfo{
		Id: roomId.String(),
	}, nil)

	mockMember.On("GetMembersByRoomId", ctx, roomId.String()).Return([]RoomMemberInfo{
		{UserId: authorId.String(), RoomId: roomId.String()},
	}, nil)

	mockUser.On("GetUser", ctx, authorId.String()).Return(&UserInfo{
		Id:   authorId.String(),
		Name: "testuser",
	}, nil)

	mockRepo.On("CreateMessage", ctx, mock.MatchedBy(func(msg domain.ChatMessage) bool {
		return msg.Content == "Hello, world!" && msg.RoomId == roomId && msg.AuthorId == authorId
	})).Return(nil)

	mockOutbox.On("Insert", ctx, mock.MatchedBy(func(ev repo.OutboxEvent) bool {
		return ev.EventType == "message_created"
	})).Return(errors.New("outbox insert error"))

	uow := makeUow(mockRepo, mockOutbox)
	svc := NewChatService(mockRepo, uow, mockUser, mockMember)
	result, err := svc.CreateMessage(ctx, roomId, req)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Equal(t, "outbox insert error", err.Error())
	mockRepo.AssertExpectations(t)
	mockOutbox.AssertExpectations(t)
}

func TestUpdateMessage_Success(t *testing.T) {
	mockRepo := new(MockChatRepo)
	mockOutbox := new(MockOutboxRepo)
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

	mockRepo.On("GetMessageById", ctx, messageId).Return(existing, nil).Once()
	mockRepo.On("UpdateMessage", ctx, mock.MatchedBy(func(msg domain.ChatMessage) bool {
		return msg.Content == "Updated content" && msg.Id == messageId
	})).Return(nil)

	mockMember.On("GetRoomById", ctx, roomId.String()).Return(&RoomInfo{
		Id: roomId.String(),
	}, nil)

	mockUser.On("GetUser", ctx, authorId.String()).Return(&UserInfo{
		Id:   authorId.String(),
		Name: "testuser",
	}, nil)

	mockOutbox.On("Insert", ctx, mock.MatchedBy(func(ev repo.OutboxEvent) bool {
		return ev.EventType == "message_updated" && ev.RoomId == roomId
	})).Return(nil)

	uow := makeUow(mockRepo, mockOutbox)
	svc := NewChatService(mockRepo, uow, mockUser, mockMember)
	result, err := svc.UpdateMessage(ctx, messageId, authorId, req)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, "Updated content", result.Content)
	mockRepo.AssertExpectations(t)
	mockOutbox.AssertExpectations(t)
	mockUser.AssertExpectations(t)
}

func TestUpdateMessage_NotAuthor(t *testing.T) {
	mockRepo := new(MockChatRepo)
	mockOutbox := new(MockOutboxRepo)
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

	mockMember.On("GetRoomById", ctx, roomId.String()).Return(&RoomInfo{
		Id: roomId.String(),
	}, nil)

	uow := makeUow(mockRepo, mockOutbox)
	svc := NewChatService(mockRepo, uow, mockUser, mockMember)
	result, err := svc.UpdateMessage(ctx, messageId, otherUserId, structs.UpdateMessageRequest{})

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Equal(t, http.StatusForbidden, err.(*errs.AppError).Status)
	mockRepo.AssertExpectations(t)
}

func TestUpdateMessage_NotFound(t *testing.T) {
	mockRepo := new(MockChatRepo)
	mockOutbox := new(MockOutboxRepo)
	mockUser := new(MockUserProvider)
	mockMember := new(MockMemberProvider)

	authorId := uuid.New()
	messageId := uuid.New()
	ctx := context.Background()

	mockRepo.On("GetMessageById", ctx, messageId).Return(domain.ChatMessage{}, errs.ErrNotFound)

	uow := makeUow(mockRepo, mockOutbox)
	svc := NewChatService(mockRepo, uow, mockUser, mockMember)
	result, err := svc.UpdateMessage(ctx, messageId, authorId, structs.UpdateMessageRequest{})

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Equal(t, errs.ErrNotFound, err)
	mockRepo.AssertExpectations(t)
}

func TestDeleteMessage_Success(t *testing.T) {
	mockRepo := new(MockChatRepo)
	mockOutbox := new(MockOutboxRepo)
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

	mockMember.On("GetRoomById", ctx, roomId.String()).Return(&RoomInfo{
		Id: roomId.String(),
	}, nil)

	mockOutbox.On("Insert", ctx, mock.MatchedBy(func(ev repo.OutboxEvent) bool {
		return ev.EventType == "message_deleted" && ev.RoomId == roomId
	})).Return(nil)

	uow := makeUow(mockRepo, mockOutbox)
	svc := NewChatService(mockRepo, uow, mockUser, mockMember)
	err := svc.DeleteMessage(ctx, messageId, authorId)

	assert.NoError(t, err)
	mockRepo.AssertExpectations(t)
	mockOutbox.AssertExpectations(t)
}

func TestDeleteMessage_NotAuthor(t *testing.T) {
	mockRepo := new(MockChatRepo)
	mockOutbox := new(MockOutboxRepo)
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

	mockMember.On("GetRoomById", ctx, roomId.String()).Return(&RoomInfo{
		Id: roomId.String(),
	}, nil)

	uow := makeUow(mockRepo, mockOutbox)
	svc := NewChatService(mockRepo, uow, mockUser, mockMember)
	err := svc.DeleteMessage(ctx, messageId, otherUserId)

	assert.Error(t, err)
	assert.Equal(t, http.StatusForbidden, err.(*errs.AppError).Status)
	mockRepo.AssertExpectations(t)
}

func TestDeleteMessage_NotFound(t *testing.T) {
	mockRepo := new(MockChatRepo)
	mockOutbox := new(MockOutboxRepo)
	mockUser := new(MockUserProvider)
	mockMember := new(MockMemberProvider)

	authorId := uuid.New()
	messageId := uuid.New()
	ctx := context.Background()

	mockRepo.On("GetMessageById", ctx, messageId).Return(domain.ChatMessage{}, errs.ErrNotFound)

	uow := makeUow(mockRepo, mockOutbox)
	svc := NewChatService(mockRepo, uow, mockUser, mockMember)
	err := svc.DeleteMessage(ctx, messageId, authorId)

	assert.Error(t, err)
	assert.Equal(t, errs.ErrNotFound, err)
	mockRepo.AssertExpectations(t)
}

func TestUpdateMessage_PoppedRoom(t *testing.T) {
	mockRepo := new(MockChatRepo)
	mockOutbox := new(MockOutboxRepo)
	mockUser := new(MockUserProvider)
	mockMember := new(MockMemberProvider)

	authorId := uuid.New()
	messageId := uuid.New()
	roomId := uuid.New()
	now := time.Now()
	ctx := context.Background()

	existing := domain.ChatMessage{
		Id:       messageId,
		RoomId:   roomId,
		AuthorId: authorId,
		Content:  "Old",
	}

	mockRepo.On("GetMessageById", ctx, messageId).Return(existing, nil)
	mockMember.On("GetRoomById", ctx, roomId.String()).Return(&RoomInfo{
		Id:       roomId.String(),
		PoppedAt: &now,
	}, nil)

	uow := makeUow(mockRepo, mockOutbox)
	svc := NewChatService(mockRepo, uow, mockUser, mockMember)
	result, err := svc.UpdateMessage(ctx, messageId, authorId, structs.UpdateMessageRequest{Content: strPtr("New")})

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Equal(t, http.StatusForbidden, err.(*errs.AppError).Status)
	assert.Equal(t, "This room has been popped and is now read-only", err.(*errs.AppError).Message)
	mockRepo.AssertExpectations(t)
	mockMember.AssertExpectations(t)
}

func TestUpdateMessage_RepoError(t *testing.T) {
	mockRepo := new(MockChatRepo)
	mockOutbox := new(MockOutboxRepo)
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
		Content:  "Old",
	}

	mockRepo.On("GetMessageById", ctx, messageId).Return(existing, nil)
	mockMember.On("GetRoomById", ctx, roomId.String()).Return(&RoomInfo{
		Id: roomId.String(),
	}, nil)
	mockUser.On("GetUser", ctx, authorId.String()).Return(&UserInfo{
		Id:   authorId.String(),
		Name: "testuser",
	}, nil)
	mockRepo.On("UpdateMessage", ctx, mock.MatchedBy(func(msg domain.ChatMessage) bool {
		return msg.Content == "New"
	})).Return(errors.New("db error"))

	uow := makeUow(mockRepo, mockOutbox)
	svc := NewChatService(mockRepo, uow, mockUser, mockMember)
	result, err := svc.UpdateMessage(ctx, messageId, authorId, structs.UpdateMessageRequest{Content: strPtr("New")})

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Equal(t, "db error", err.Error())
	mockRepo.AssertExpectations(t)
}

func TestDeleteMessage_PoppedRoom(t *testing.T) {
	mockRepo := new(MockChatRepo)
	mockOutbox := new(MockOutboxRepo)
	mockUser := new(MockUserProvider)
	mockMember := new(MockMemberProvider)

	authorId := uuid.New()
	messageId := uuid.New()
	roomId := uuid.New()
	now := time.Now()
	ctx := context.Background()

	existing := domain.ChatMessage{
		Id:       messageId,
		RoomId:   roomId,
		AuthorId: authorId,
	}

	mockRepo.On("GetMessageById", ctx, messageId).Return(existing, nil)
	mockMember.On("GetRoomById", ctx, roomId.String()).Return(&RoomInfo{
		Id:       roomId.String(),
		PoppedAt: &now,
	}, nil)

	uow := makeUow(mockRepo, mockOutbox)
	svc := NewChatService(mockRepo, uow, mockUser, mockMember)
	err := svc.DeleteMessage(ctx, messageId, authorId)

	assert.Error(t, err)
	assert.Equal(t, http.StatusForbidden, err.(*errs.AppError).Status)
	assert.Equal(t, "This room has been popped and is now read-only", err.(*errs.AppError).Message)
	mockRepo.AssertExpectations(t)
	mockMember.AssertExpectations(t)
}

func TestDeleteMessage_RepoError(t *testing.T) {
	mockRepo := new(MockChatRepo)
	mockOutbox := new(MockOutboxRepo)
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
	}

	mockRepo.On("GetMessageById", ctx, messageId).Return(existing, nil)
	mockMember.On("GetRoomById", ctx, roomId.String()).Return(&RoomInfo{
		Id: roomId.String(),
	}, nil)
	mockRepo.On("DeleteMessage", ctx, messageId).Return(errors.New("db error"))

	uow := makeUow(mockRepo, mockOutbox)
	svc := NewChatService(mockRepo, uow, mockUser, mockMember)
	err := svc.DeleteMessage(ctx, messageId, authorId)

	assert.Error(t, err)
	assert.Equal(t, "db error", err.Error())
	mockRepo.AssertExpectations(t)
}

func TestCreateMessage_PoppedRoom(t *testing.T) {
	mockRepo := new(MockChatRepo)
	mockOutbox := new(MockOutboxRepo)
	mockUser := new(MockUserProvider)
	mockMember := new(MockMemberProvider)

	authorId := uuid.New()
	roomId := uuid.New()
	now := time.Now()
	ctx := context.WithValue(context.Background(), "authorId", authorId)

	req := structs.CreateMessageRequest{
		MessageType: domain.MessageTypeTEXT,
		Content:     "Hello",
	}

	mockMember.On("GetRoomById", ctx, roomId.String()).Return(&RoomInfo{
		Id:       roomId.String(),
		PoppedAt: &now,
	}, nil)

	uow := makeUow(mockRepo, mockOutbox)
	svc := NewChatService(mockRepo, uow, mockUser, mockMember)
	result, err := svc.CreateMessage(ctx, roomId, req)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Equal(t, http.StatusForbidden, err.(*errs.AppError).Status)
	assert.Equal(t, "This room has been popped and is now read-only", err.(*errs.AppError).Message)
	mockRepo.AssertNotCalled(t, "CreateMessage", mock.Anything, mock.Anything)
	mockMember.AssertExpectations(t)
}

func TestCreateMessage_NonExistentRoom(t *testing.T) {
	mockRepo := new(MockChatRepo)
	mockOutbox := new(MockOutboxRepo)
	mockUser := new(MockUserProvider)
	mockMember := new(MockMemberProvider)

	authorId := uuid.New()
	roomId := uuid.New()
	ctx := context.WithValue(context.Background(), "authorId", authorId)

	req := structs.CreateMessageRequest{
		MessageType: domain.MessageTypeTEXT,
		Content:     "Hello",
	}

	mockMember.On("GetRoomById", ctx, roomId.String()).Return(nil, errs.ErrNotFound)

	uow := makeUow(mockRepo, mockOutbox)
	svc := NewChatService(mockRepo, uow, mockUser, mockMember)
	result, err := svc.CreateMessage(ctx, roomId, req)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Equal(t, errs.ErrNotFound, err)
	mockRepo.AssertNotCalled(t, "CreateMessage", mock.Anything, mock.Anything)
	mockMember.AssertExpectations(t)
}

func strPtr(s string) *string {
	return &s
}
