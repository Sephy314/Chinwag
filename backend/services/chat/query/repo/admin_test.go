package repo

import (
	"context"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestProjectionRepo(t *testing.T) (*ProjectionRepo, sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	return NewProjectionRepo(sqlx.NewDb(db, "sqlmock")), mock
}

func TestProjectionRepo_AdminListMessages_NoFilters(t *testing.T) {
	repo, mock := newTestProjectionRepo(t)
	now := time.Now().Truncate(0)
	id := uuid.New()
	roomId := uuid.New()
	authorId := uuid.New()
	rows := sqlmock.NewRows([]string{"id", "room_id", "author_id", "author_name", "message_type", "content", "created_at", "updated_at", "deleted_at"}).
		AddRow(id, roomId, authorId, "alice", 0, "hello", now, nil, nil)
	mock.ExpectQuery(`SELECT id, room_id, author_id, author_name, message_type, content, created_at, updated_at, deleted_at FROM message_projections WHERE deleted_at IS NULL ORDER BY created_at DESC, id DESC LIMIT \$1`).
		WithArgs(51).
		WillReturnRows(rows)

	msgs, meta, err := repo.AdminListMessages(context.Background(), "", 50, nil, nil, "")
	assert.NoError(t, err)
	assert.Len(t, msgs, 1)
	assert.Nil(t, meta)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestProjectionRepo_AdminListMessages_AllFilters(t *testing.T) {
	repo, mock := newTestProjectionRepo(t)
	now := time.Now().Truncate(0)
	id := uuid.New()
	roomId := uuid.New()
	authorId := uuid.New()
	rows := sqlmock.NewRows([]string{"id", "room_id", "author_id", "author_name", "message_type", "content", "created_at", "updated_at", "deleted_at"}).
		AddRow(id, roomId, authorId, "alice", 0, "hello", now, nil, nil)
	mock.ExpectQuery(`SELECT id, room_id, author_id, author_name, message_type, content, created_at, updated_at, deleted_at FROM message_projections WHERE deleted_at IS NULL AND room_id = \$1 AND author_id = \$2 AND content ILIKE \$3 ORDER BY created_at DESC, id DESC LIMIT \$4`).
		WithArgs(roomId, authorId, "%chat%", 51).
		WillReturnRows(rows)

	msgs, _, err := repo.AdminListMessages(context.Background(), "", 50, &roomId, &authorId, "chat")
	assert.NoError(t, err)
	assert.Len(t, msgs, 1)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestProjectionRepo_AdminListMessages_WithCursor(t *testing.T) {
	repo, mock := newTestProjectionRepo(t)
	now := time.Now().Truncate(0).UTC()
	id := uuid.New()
	roomId := uuid.New()
	authorId := uuid.New()
	cursor := encodeCursor(now, id)
	rows := sqlmock.NewRows([]string{"id", "room_id", "author_id", "author_name", "message_type", "content", "created_at", "updated_at", "deleted_at"}).
		AddRow(id, roomId, authorId, "alice", 0, "hello", now, nil, nil)
	mock.ExpectQuery(`SELECT id, room_id, author_id, author_name, message_type, content, created_at, updated_at, deleted_at FROM message_projections WHERE deleted_at IS NULL AND \(created_at, id\) < \(\$1, \$2\) ORDER BY created_at DESC, id DESC LIMIT \$3`).
		WithArgs(now, id, 51).
		WillReturnRows(rows)

	msgs, _, err := repo.AdminListMessages(context.Background(), cursor, 50, nil, nil, "")
	assert.NoError(t, err)
	assert.Len(t, msgs, 1)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestProjectionRepo_AdminListMessages_InvalidCursor(t *testing.T) {
	repo, _ := newTestProjectionRepo(t)
	_, _, err := repo.AdminListMessages(context.Background(), "bad!!", 50, nil, nil, "")
	assert.Error(t, err)
}

func TestProjectionRepo_AdminGetMessageIncludingDeleted(t *testing.T) {
	repo, mock := newTestProjectionRepo(t)
	now := time.Now().Truncate(0)
	id := uuid.New()
	roomId := uuid.New()
	authorId := uuid.New()
	deletedAt := now.Add(-time.Hour)
	rows := sqlmock.NewRows([]string{"id", "room_id", "author_id", "author_name", "message_type", "content", "created_at", "updated_at", "deleted_at"}).
		AddRow(id, roomId, authorId, "alice", 0, "gone", now, nil, deletedAt)
	mock.ExpectQuery(`SELECT id, room_id, author_id, author_name, message_type, content, created_at, updated_at, deleted_at FROM message_projections WHERE id = \$1`).
		WithArgs(id).
		WillReturnRows(rows)

	msg, err := repo.AdminGetMessageIncludingDeleted(context.Background(), id)
	assert.NoError(t, err)
	assert.Equal(t, id, msg.Id)
	assert.NotNil(t, msg.DeletedAt)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestProjectionRepo_AdminCountMessages(t *testing.T) {
	repo, mock := newTestProjectionRepo(t)
	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM message_projections WHERE deleted_at IS NULL`).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(9))

	n, err := repo.AdminCountMessages(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, 9, n)
	require.NoError(t, mock.ExpectationsWereMet())
}
