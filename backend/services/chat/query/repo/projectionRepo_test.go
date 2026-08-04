package repo

import (
	"context"
	"testing"
	"time"

	"github.com/Sephy314/chinwag/backend/services/chat/query/domain"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/assert"
)

func setupTestRepo(t *testing.T) (*ProjectionRepo, func()) {
	db, err := sqlx.Connect("pgx", "postgres://sephy:ouilala0328@localhost:5432/chinwag_chat_projection_test?sslmode=disable")
	if err != nil {
		t.Skip("test database not available")
	}

	db.MustExec(`TRUNCATE TABLE message_projections`)

	repo := NewProjectionRepo(db)
	return repo, func() { db.Close() }
}

func TestUpsert_CreatesNewRecord(t *testing.T) {
	repo, cleanup := setupTestRepo(t)
	defer cleanup()

	msg := domain.MessageProjection{
		Id:          uuid.New(),
		RoomId:      uuid.New(),
		AuthorId:    uuid.New(),
		AuthorName:  "testuser",
		MessageType: 0,
		Content:     "Hello, world!",
		CreatedAt:   time.Now(),
	}

	err := repo.Upsert(context.Background(), msg)
	assert.NoError(t, err)

	fetched, err := repo.GetById(context.Background(), msg.Id)
	assert.NoError(t, err)
	assert.Equal(t, msg.Content, fetched.Content)
	assert.Equal(t, msg.AuthorName, fetched.AuthorName)
}

func TestUpsert_IdempotentDuplicate(t *testing.T) {
	repo, cleanup := setupTestRepo(t)
	defer cleanup()

	msg := domain.MessageProjection{
		Id:          uuid.New(),
		RoomId:      uuid.New(),
		AuthorId:    uuid.New(),
		AuthorName:  "testuser",
		MessageType: 0,
		Content:     "Original",
		CreatedAt:   time.Now(),
	}

	err := repo.Upsert(context.Background(), msg)
	assert.NoError(t, err)

	msg.Content = "Updated"
	err = repo.Upsert(context.Background(), msg)
	assert.NoError(t, err)

	fetched, err := repo.GetById(context.Background(), msg.Id)
	assert.NoError(t, err)
	assert.Equal(t, "Updated", fetched.Content)
}

func TestGetById_NotFound(t *testing.T) {
	repo, cleanup := setupTestRepo(t)
	defer cleanup()

	_, err := repo.GetById(context.Background(), uuid.New())
	assert.Error(t, err)
}

func TestListByRoomId_Empty(t *testing.T) {
	repo, cleanup := setupTestRepo(t)
	defer cleanup()

	msgs, meta, err := repo.ListByRoomId(context.Background(), uuid.New(), "", 50)
	assert.NoError(t, err)
	assert.Empty(t, msgs)
	assert.Nil(t, meta)
}

func TestListByRoomId_WithMessages(t *testing.T) {
	repo, cleanup := setupTestRepo(t)
	defer cleanup()

	roomId := uuid.New()
	now := time.Now()

	for i := 0; i < 3; i++ {
		msg := domain.MessageProjection{
			Id:          uuid.New(),
			RoomId:      roomId,
			AuthorId:    uuid.New(),
			AuthorName:  "user",
			MessageType: 0,
			Content:     "Message",
			CreatedAt:   now.Add(-time.Duration(i) * time.Minute),
		}
		if err := repo.Upsert(context.Background(), msg); err != nil {
			t.Fatalf("failed to upsert message: %v", err)
		}
	}

	msgs, meta, err := repo.ListByRoomId(context.Background(), roomId, "", 50)
	assert.NoError(t, err)
	assert.Len(t, msgs, 3)
	assert.Nil(t, meta)
}

func TestListByRoomId_CursorPagination(t *testing.T) {
	repo, cleanup := setupTestRepo(t)
	defer cleanup()

	roomId := uuid.New()
	now := time.Now()

	for i := 0; i < 5; i++ {
		msg := domain.MessageProjection{
			Id:          uuid.New(),
			RoomId:      roomId,
			AuthorId:    uuid.New(),
			AuthorName:  "user",
			MessageType: 0,
			Content:     "Message",
			CreatedAt:   now.Add(-time.Duration(i) * time.Minute),
		}
		if err := repo.Upsert(context.Background(), msg); err != nil {
			t.Fatalf("failed to upsert message: %v", err)
		}
	}

	msgs, meta, err := repo.ListByRoomId(context.Background(), roomId, "", 2)
	assert.NoError(t, err)
	assert.Len(t, msgs, 2)
	assert.NotNil(t, meta)
	assert.True(t, meta.HasMore)
	assert.NotEmpty(t, meta.NextCursor)

	msgs2, meta2, err := repo.ListByRoomId(context.Background(), roomId, meta.NextCursor, 2)
	assert.NoError(t, err)
	assert.Len(t, msgs2, 2)
	assert.NotNil(t, meta2)
	assert.True(t, meta2.HasMore)
}

func TestUpdateContent(t *testing.T) {
	repo, cleanup := setupTestRepo(t)
	defer cleanup()

	msg := domain.MessageProjection{
		Id:          uuid.New(),
		RoomId:      uuid.New(),
		AuthorId:    uuid.New(),
		AuthorName:  "user",
		MessageType: 0,
		Content:     "Original",
		CreatedAt:   time.Now(),
	}
	repo.Upsert(context.Background(), msg)

	now := time.Now()
	err := repo.UpdateContent(context.Background(), msg.Id, "Updated", now)
	assert.NoError(t, err)

	fetched, err := repo.GetById(context.Background(), msg.Id)
	assert.NoError(t, err)
	assert.Equal(t, "Updated", fetched.Content)
	assert.NotNil(t, fetched.UpdatedAt)
}

func TestUpdateContent_NotFound(t *testing.T) {
	repo, cleanup := setupTestRepo(t)
	defer cleanup()

	err := repo.UpdateContent(context.Background(), uuid.New(), "test", time.Now())
	assert.Error(t, err)
}

func TestSoftDelete(t *testing.T) {
	repo, cleanup := setupTestRepo(t)
	defer cleanup()

	msg := domain.MessageProjection{
		Id:          uuid.New(),
		RoomId:      uuid.New(),
		AuthorId:    uuid.New(),
		AuthorName:  "user",
		MessageType: 0,
		Content:     "To delete",
		CreatedAt:   time.Now(),
	}
	repo.Upsert(context.Background(), msg)

	err := repo.SoftDelete(context.Background(), msg.Id, time.Now())
	assert.NoError(t, err)

	_, err = repo.GetById(context.Background(), msg.Id)
	assert.Error(t, err)
}

func TestSoftDelete_NotFound(t *testing.T) {
	repo, cleanup := setupTestRepo(t)
	defer cleanup()

	err := repo.SoftDelete(context.Background(), uuid.New(), time.Now())
	assert.Error(t, err)
}

func TestSoftDelete_Idempotent(t *testing.T) {
	repo, cleanup := setupTestRepo(t)
	defer cleanup()

	msg := domain.MessageProjection{
		Id:          uuid.New(),
		RoomId:      uuid.New(),
		AuthorId:    uuid.New(),
		AuthorName:  "user",
		MessageType: 0,
		Content:     "Test",
		CreatedAt:   time.Now(),
	}
	repo.Upsert(context.Background(), msg)

	err := repo.SoftDelete(context.Background(), msg.Id, time.Now())
	assert.NoError(t, err)

	err = repo.SoftDelete(context.Background(), msg.Id, time.Now())
	assert.Error(t, err)
}

var _ ProjectionRepoInterface = (*ProjectionRepo)(nil)
