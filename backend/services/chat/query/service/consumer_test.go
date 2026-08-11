package service

import (
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/Sephy314/chinwag/backend/services/chat/query/domain"
	"github.com/Sephy314/chinwag/backend/services/chat/query/structs"
	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
)

type MockProjectionRepoConsumer struct {
	mock.Mock
}

func (m *MockProjectionRepoConsumer) Upsert(ctx context.Context, msg domain.MessageProjection) error {
	args := m.Called(ctx, msg)
	return args.Error(0)
}

func (m *MockProjectionRepoConsumer) GetById(ctx context.Context, id uuid.UUID) (domain.MessageProjection, error) {
	args := m.Called(ctx, id)
	return args.Get(0).(domain.MessageProjection), args.Error(1)
}

func (m *MockProjectionRepoConsumer) ListByRoomId(ctx context.Context, roomId uuid.UUID, cursorStr string, limit int) ([]domain.MessageProjection, *structs.CursorMeta, error) {
	args := m.Called(ctx, roomId, cursorStr, limit)
	return args.Get(0).([]domain.MessageProjection), args.Get(1).(*structs.CursorMeta), args.Error(2)
}

func (m *MockProjectionRepoConsumer) ListAfterByRoomId(ctx context.Context, roomId uuid.UUID, afterCursor string, limit int) ([]domain.MessageProjection, error) {
	args := m.Called(ctx, roomId, afterCursor, limit)
	return args.Get(0).([]domain.MessageProjection), args.Error(1)
}

func (m *MockProjectionRepoConsumer) UpdateContent(ctx context.Context, id uuid.UUID, content string, updatedAt time.Time) error {
	args := m.Called(ctx, id, content, updatedAt)
	return args.Error(0)
}

func (m *MockProjectionRepoConsumer) SoftDelete(ctx context.Context, id uuid.UUID, deletedAt time.Time) error {
	args := m.Called(ctx, id, deletedAt)
	return args.Error(0)
}

func (m *MockProjectionRepoConsumer) AdminListMessages(ctx context.Context, cursorStr string, limit int, roomID, authorID *uuid.UUID, search string) ([]domain.MessageProjection, *structs.CursorMeta, error) {
	args := m.Called(ctx, cursorStr, limit, roomID, authorID, search)
	return args.Get(0).([]domain.MessageProjection), args.Get(1).(*structs.CursorMeta), args.Error(2)
}

func (m *MockProjectionRepoConsumer) AdminGetMessageIncludingDeleted(ctx context.Context, id uuid.UUID) (domain.MessageProjection, error) {
	args := m.Called(ctx, id)
	return args.Get(0).(domain.MessageProjection), args.Error(1)
}

func (m *MockProjectionRepoConsumer) AdminCountMessages(ctx context.Context) (int, error) {
	args := m.Called(ctx)
	return args.Int(0), args.Error(1)
}

func log() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stdout, nil))
}

func TestConsumer_HandleCreated(t *testing.T) {
	mockRepo := new(MockProjectionRepoConsumer)
	consumer := NewProjectionConsumer(mockRepo, log())

	msgId := uuid.New()
	roomId := uuid.New()
	authorId := uuid.New()
	now := time.Now().UTC().Format(time.RFC3339Nano)

	payload, _ := json.Marshal(map[string]any{
		"type": "new_message",
		"data": map[string]any{
			"id":           msgId.String(),
			"room_id":      roomId.String(),
			"author_id":    authorId.String(),
			"author_name":  "testuser",
			"message_type": float64(0),
			"content":      "Hello!",
			"created_at":   now,
		},
	})

	mockRepo.On("Upsert", mock.Anything, mock.MatchedBy(func(msg domain.MessageProjection) bool {
		return msg.Content == "Hello!" && msg.AuthorName == "testuser"
	})).Return(nil)

	consumer.Handle(roomId, payload)

	mockRepo.AssertExpectations(t)
}

func TestConsumer_HandleUpdated(t *testing.T) {
	mockRepo := new(MockProjectionRepoConsumer)
	consumer := NewProjectionConsumer(mockRepo, log())

	msgId := uuid.New()
	roomId := uuid.New()
	now := time.Now().UTC().Format(time.RFC3339Nano)

	payload, _ := json.Marshal(map[string]any{
		"type": "updated_message",
		"data": map[string]any{
			"id":         msgId.String(),
			"room_id":    roomId.String(),
			"content":    "Updated content",
			"updated_at": now,
		},
	})

	parsedTime, _ := time.Parse(time.RFC3339Nano, now)

	mockRepo.On("UpdateContent", mock.Anything, msgId, "Updated content", mock.MatchedBy(func(t time.Time) bool {
		return t.Equal(parsedTime)
	})).Return(nil)

	consumer.Handle(roomId, payload)

	mockRepo.AssertExpectations(t)
}

func TestConsumer_HandleDeleted(t *testing.T) {
	mockRepo := new(MockProjectionRepoConsumer)
	consumer := NewProjectionConsumer(mockRepo, log())

	msgId := uuid.New()
	roomId := uuid.New()

	payload, _ := json.Marshal(map[string]any{
		"type": "deleted_message",
		"data": map[string]any{
			"id":      msgId.String(),
			"room_id": roomId.String(),
		},
	})

	mockRepo.On("SoftDelete", mock.Anything, msgId, mock.AnythingOfType("time.Time")).Return(nil)

	consumer.Handle(roomId, payload)

	mockRepo.AssertExpectations(t)
}

func TestConsumer_HandleDuplicateCreate(t *testing.T) {
	mockRepo := new(MockProjectionRepoConsumer)
	consumer := NewProjectionConsumer(mockRepo, log())

	msgId := uuid.New()
	roomId := uuid.New()
	authorId := uuid.New()
	now := time.Now().UTC().Format(time.RFC3339Nano)

	payload, _ := json.Marshal(map[string]any{
		"type": "new_message",
		"data": map[string]any{
			"id":           msgId.String(),
			"room_id":      roomId.String(),
			"author_id":    authorId.String(),
			"author_name":  "testuser",
			"message_type": float64(0),
			"content":      "Hello!",
			"created_at":   now,
		},
	})

	// Same event delivered twice
	mockRepo.On("Upsert", mock.Anything, mock.Anything).Return(nil).Twice()

	consumer.Handle(roomId, payload)
	consumer.Handle(roomId, payload)

	mockRepo.AssertNumberOfCalls(t, "Upsert", 2)
}

func TestConsumer_HandleUnknownEventType(t *testing.T) {
	mockRepo := new(MockProjectionRepoConsumer)
	consumer := NewProjectionConsumer(mockRepo, log())

	payload, _ := json.Marshal(map[string]any{
		"type": "unknown_type",
		"data": map[string]any{},
	})

	// Should not panic, just log a warning
	consumer.Handle(uuid.New(), payload)
}

func TestConsumer_HandleMalformedData(t *testing.T) {
	mockRepo := new(MockProjectionRepoConsumer)
	consumer := NewProjectionConsumer(mockRepo, log())

	// Not JSON
	consumer.Handle(uuid.New(), []byte("not json"))

	// Should not panic
}

func TestConsumer_HandleEmptyData(t *testing.T) {
	mockRepo := new(MockProjectionRepoConsumer)
	consumer := NewProjectionConsumer(mockRepo, log())

	payload, _ := json.Marshal(map[string]any{
		"type": "new_message",
		"data": map[string]any{
			"id":           "invalid-uuid",
			"room_id":      "invalid-uuid",
			"author_id":    "invalid-uuid",
			"author_name":  "test",
			"message_type": float64(0),
			"content":      "test",
			"created_at":   "invalid-date",
		},
	})

	consumer.Handle(uuid.New(), payload)
	// Should not panic
}
