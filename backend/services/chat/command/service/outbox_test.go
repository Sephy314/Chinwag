package service

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/Sephy314/chinwag/backend/services/chat/command/repo"
	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
)

type MockRawNatsPublisher struct {
	mock.Mock
}

func (m *MockRawNatsPublisher) PublishRaw(ctx context.Context, subject string, data []byte) error {
	args := m.Called(ctx, subject, data)
	return args.Error(0)
}

func mockOutboxEvent(id uuid.UUID) repo.OutboxEvent {
	return repo.OutboxEvent{
		Id:        id,
		EventType: "message_created",
		Subject:   "chat.room.test-room",
		Payload:   []byte(`{"type":"new_message","data":{"content":"hello"}}`),
		RoomId:    uuid.New(),
		CreatedAt: time.Now(),
	}
}

func TestOutboxPublisher_PublishBatch(t *testing.T) {
	mockOutbox := new(MockOutboxRepo)
	mockNats := new(MockRawNatsPublisher)
	log := testLogger(t)

	evtID := uuid.New()
	evt := mockOutboxEvent(evtID)

	mockOutbox.On("PollPending", mock.Anything, 50).Return([]repo.OutboxEvent{evt}, nil)
	mockNats.On("PublishRaw", mock.Anything, evt.Subject, evt.Payload).Return(nil)
	mockOutbox.On("MarkPublished", mock.Anything, evtID).Return(nil)

	publisher := NewOutboxPublisher(mockOutbox, mockNats, log)

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	publisher.processBatch(ctx)

	mockOutbox.AssertExpectations(t)
	mockNats.AssertExpectations(t)
}

func TestOutboxPublisher_PublishFailure_IncrementsRetry(t *testing.T) {
	mockOutbox := new(MockOutboxRepo)
	mockNats := new(MockRawNatsPublisher)
	log := testLogger(t)

	evtID := uuid.New()
	evt := mockOutboxEvent(evtID)

	mockOutbox.On("PollPending", mock.Anything, 50).Return([]repo.OutboxEvent{evt}, nil)
	mockNats.On("PublishRaw", mock.Anything, evt.Subject, evt.Payload).Return(errors.New("nats error"))
	mockOutbox.On("IncrementRetry", mock.Anything, evtID).Return(nil)
	mockOutbox.AssertNotCalled(t, "MarkPublished", mock.Anything, mock.Anything)

	publisher := NewOutboxPublisher(mockOutbox, mockNats, log)

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	publisher.processBatch(ctx)

	mockOutbox.AssertExpectations(t)
	mockNats.AssertExpectations(t)
}

func TestOutboxPublisher_PollFailure(t *testing.T) {
	mockOutbox := new(MockOutboxRepo)
	mockNats := new(MockRawNatsPublisher)
	log := testLogger(t)

	mockOutbox.On("PollPending", mock.Anything, 50).Return(nil, errors.New("db error"))
	mockNats.AssertNotCalled(t, "PublishRaw", mock.Anything, mock.Anything, mock.Anything)

	publisher := NewOutboxPublisher(mockOutbox, mockNats, log)

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	publisher.processBatch(ctx)

	mockOutbox.AssertExpectations(t)
	mockNats.AssertExpectations(t)
}

func TestOutboxPublisher_MarkFailure_DoesNotBlock(t *testing.T) {
	mockOutbox := new(MockOutboxRepo)
	mockNats := new(MockRawNatsPublisher)
	log := testLogger(t)

	evtID := uuid.New()
	evt := mockOutboxEvent(evtID)

	mockOutbox.On("PollPending", mock.Anything, 50).Return([]repo.OutboxEvent{evt}, nil)
	mockNats.On("PublishRaw", mock.Anything, evt.Subject, evt.Payload).Return(nil)
	mockOutbox.On("MarkPublished", mock.Anything, evtID).Return(errors.New("mark error"))

	publisher := NewOutboxPublisher(mockOutbox, mockNats, log)

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	publisher.processBatch(ctx)

	mockOutbox.AssertExpectations(t)
	mockNats.AssertExpectations(t)
}

func testLogger(t *testing.T) *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}
