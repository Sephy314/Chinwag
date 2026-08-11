package service

import (
	"context"
	"testing"
	"time"

	"github.com/Sephy314/chinwag/backend/services/chat/query/domain"
	"github.com/Sephy314/chinwag/backend/services/chat/query/structs"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestQueryService_AdminListMessages_Success(t *testing.T) {
	mockRepo := new(MockProjectionRepo)
	svc := NewQueryService(mockRepo, new(MockMemberProvider), noopCache{})

	roomId := uuid.New()
	authorId := uuid.New()
	now := time.Now()
	msgs := []domain.MessageProjection{{
		Id: uuid.New(), RoomId: roomId, AuthorId: authorId, AuthorName: "alice",
		MessageType: 0, Content: "hello", CreatedAt: now,
	}}
	mockRepo.On("AdminListMessages", mock.Anything, "cur", 25, (*uuid.UUID)(nil), (*uuid.UUID)(nil), "").
		Return(msgs, (*structs.CursorMeta)(nil), nil).Once()

	got, _, err := svc.AdminListMessages(context.Background(), structs.AdminListMessagesRequest{Cursor: "cur", Limit: 25})
	assert.NoError(t, err)
	assert.Len(t, got, 1)
	assert.Equal(t, "alice", got[0].AuthorName)
	mockRepo.AssertExpectations(t)
}

func TestQueryService_AdminListMessages_InvalidRoomID(t *testing.T) {
	mockRepo := new(MockProjectionRepo)
	svc := NewQueryService(mockRepo, new(MockMemberProvider), noopCache{})

	_, _, err := svc.AdminListMessages(context.Background(), structs.AdminListMessagesRequest{RoomID: "not-a-uuid"})
	assert.Error(t, err)
	mockRepo.AssertNotCalled(t, "AdminListMessages", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

func TestQueryService_AdminGetMessage_ReturnsDeleted(t *testing.T) {
	mockRepo := new(MockProjectionRepo)
	svc := NewQueryService(mockRepo, new(MockMemberProvider), noopCache{})

	messageId := uuid.New()
	msg := domain.MessageProjection{Id: messageId, AuthorName: "bob", Content: "gone"}
	mockRepo.On("AdminGetMessageIncludingDeleted", mock.Anything, messageId).Return(msg, nil).Once()

	got, err := svc.AdminGetMessage(context.Background(), messageId)
	assert.NoError(t, err)
	assert.NotNil(t, got)
	assert.Equal(t, "gone", got.Content)
	mockRepo.AssertExpectations(t)
}

func TestQueryService_AdminCountMessages(t *testing.T) {
	mockRepo := new(MockProjectionRepo)
	svc := NewQueryService(mockRepo, new(MockMemberProvider), noopCache{})

	mockRepo.On("AdminCountMessages", mock.Anything).Return(77, nil).Once()

	n, err := svc.AdminCountMessages(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, int64(77), n)
	mockRepo.AssertExpectations(t)
}
