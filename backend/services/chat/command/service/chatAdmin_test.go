package service

import (
	"context"
	"errors"
	"testing"

	"github.com/Sephy314/chinwag/backend/services/chat/command/domain"
	"github.com/Sephy314/chinwag/backend/services/chat/command/repo"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestChatService_AdminDeleteMessage_Success(t *testing.T) {
	mockRepo := new(MockChatRepo)
	mockOutbox := new(MockOutboxRepo)
	svc := NewChatService(mockRepo, makeUow(mockRepo, mockOutbox), new(MockUserProvider), new(MockMemberProvider))

	messageId := uuid.New()
	roomId := uuid.New()

	mockRepo.On("GetMessageById", mock.Anything, messageId).Return(domain.ChatMessage{
		Id:     messageId,
		RoomId: roomId,
	}, nil).Once()
	mockRepo.On("DeleteMessage", mock.Anything, messageId).Return(nil).Once()
	mockOutbox.On("Insert", mock.Anything, mock.MatchedBy(func(ev repo.OutboxEvent) bool {
		return ev.EventType == "message_deleted" && ev.RoomId == roomId && ev.Subject == "chat.room."+roomId.String()
	})).Return(nil).Once()

	err := svc.AdminDeleteMessage(context.Background(), messageId)
	assert.NoError(t, err)
	mockRepo.AssertExpectations(t)
	mockOutbox.AssertExpectations(t)
}

func TestChatService_AdminDeleteMessage_GetFails(t *testing.T) {
	mockRepo := new(MockChatRepo)
	svc := NewChatService(mockRepo, makeUow(mockRepo, new(MockOutboxRepo)), new(MockUserProvider), new(MockMemberProvider))

	messageId := uuid.New()
	mockRepo.On("GetMessageById", mock.Anything, messageId).Return(domain.ChatMessage{}, errors.New("not found")).Once()

	err := svc.AdminDeleteMessage(context.Background(), messageId)
	assert.Error(t, err)
	mockRepo.AssertNotCalled(t, "DeleteMessage", mock.Anything, mock.Anything)
}
