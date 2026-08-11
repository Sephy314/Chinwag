package repo

import (
	"context"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Sephy314/chinwag/backend/services/room/domain"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestRoomRepo(t *testing.T) (*RoomRepo, sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	return NewRoomRepo(sqlx.NewDb(db, "sqlmock")), mock
}

func TestRoomRepo_AdminUpdateRoom_AllowsPopped(t *testing.T) {
	repo, mock := newTestRoomRepo(t)
	room := domain.Room{
		Id:          uuid.MustParse("11111111-1111-1111-1111-111111111111"),
		Name:        "updated",
		Description: ptrStr("d"),
		MaxMembers:  10,
	}
	mock.ExpectExec(`UPDATE rooms SET name = \$1, description = \$2, max_members = \$3, updated_at = NOW\(\) WHERE id = \$4 AND deleted_at IS NULL`).
		WithArgs(room.Name, room.Description, room.MaxMembers, room.Id).
		WillReturnResult(sqlmock.NewResult(1, 1))

	err := repo.AdminUpdateRoom(context.Background(), room)
	assert.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestRoomRepo_AdminUpdateRoom_NotFound(t *testing.T) {
	repo, mock := newTestRoomRepo(t)
	room := domain.Room{Id: uuid.MustParse("11111111-1111-1111-1111-111111111111")}
	mock.ExpectExec(`UPDATE rooms SET name = .* deleted_at IS NULL`).
		WithArgs(room.Name, room.Description, room.MaxMembers, room.Id).
		WillReturnResult(sqlmock.NewResult(0, 0))

	err := repo.AdminUpdateRoom(context.Background(), room)
	assert.Error(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestRoomRepo_AdminDeleteRoomById_AllowsPopped(t *testing.T) {
	repo, mock := newTestRoomRepo(t)
	id := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	mock.ExpectExec(`UPDATE rooms SET deleted_at = now\(\) WHERE id = \$1`).
		WithArgs(id).
		WillReturnResult(sqlmock.NewResult(1, 1))

	err := repo.AdminDeleteRoomById(context.Background(), id)
	assert.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestRoomRepo_AdminDeleteRoomById_NotFound(t *testing.T) {
	repo, mock := newTestRoomRepo(t)
	id := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	mock.ExpectExec(`UPDATE rooms SET deleted_at = now\(\) WHERE id = \$1`).
		WithArgs(id).
		WillReturnResult(sqlmock.NewResult(0, 0))

	err := repo.AdminDeleteRoomById(context.Background(), id)
	assert.Error(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestRoomRepo_CountRooms(t *testing.T) {
	repo, mock := newTestRoomRepo(t)
	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM rooms WHERE deleted_at IS NULL`).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(7))

	n, err := repo.CountRooms(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, 7, n)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestRoomRepo_ListRooms_NoCursor(t *testing.T) {
	repo, mock := newTestRoomRepo(t)
	now := time.Now()
	id := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	rows := sqlmock.NewRows([]string{"id", "name", "description", "max_members", "owner_id", "pop_at", "popped_at", "created_at", "updated_at", "deleted_at"}).
		AddRow(id, "room1", nil, 20, id, nil, nil, now, now, nil)
	mock.ExpectQuery(`SELECT id, name, description, max_members, owner_id, pop_at, popped_at, created_at, updated_at, deleted_at FROM rooms WHERE deleted_at IS NULL ORDER BY created_at DESC, id DESC LIMIT \$1`).
		WithArgs(51).
		WillReturnRows(rows)

	rooms, meta, err := repo.ListRooms(context.Background(), "", 50, "")
	assert.NoError(t, err)
	assert.Len(t, rooms, 1)
	assert.Nil(t, meta)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestRoomRepo_ListRooms_SearchNoCursor(t *testing.T) {
	repo, mock := newTestRoomRepo(t)
	now := time.Now()
	id := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	rows := sqlmock.NewRows([]string{"id", "name", "description", "max_members", "owner_id", "pop_at", "popped_at", "created_at", "updated_at", "deleted_at"}).
		AddRow(id, "room1", nil, 20, id, nil, nil, now, now, nil)
	mock.ExpectQuery(`SELECT id, name, description, max_members, owner_id, pop_at, popped_at, created_at, updated_at, deleted_at FROM rooms WHERE deleted_at IS NULL AND name ILIKE \$1 ORDER BY created_at DESC, id DESC LIMIT \$2`).
		WithArgs("%chat%", 51).
		WillReturnRows(rows)

	rooms, _, err := repo.ListRooms(context.Background(), "", 50, "chat")
	assert.NoError(t, err)
	assert.Len(t, rooms, 1)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestRoomRepo_ListRooms_WithCursor(t *testing.T) {
	repo, mock := newTestRoomRepo(t)
	now := time.Now().Truncate(0).UTC() // strip monotonic + force UTC so the JSON round-trip DeepEqual matches on any TZ
	id := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	cursor := encodeRoomCursor(now, id)
	rows := sqlmock.NewRows([]string{"id", "name", "description", "max_members", "owner_id", "pop_at", "popped_at", "created_at", "updated_at", "deleted_at"}).
		AddRow(id, "room1", nil, 20, id, nil, nil, now, now, nil)
	mock.ExpectQuery(`SELECT id, name, description, max_members, owner_id, pop_at, popped_at, created_at, updated_at, deleted_at FROM rooms WHERE deleted_at IS NULL AND \(created_at, id\) < \(\$1, \$2\) ORDER BY created_at DESC, id DESC LIMIT \$3`).
		WithArgs(now, id, 51).
		WillReturnRows(rows)

	rooms, _, err := repo.ListRooms(context.Background(), cursor, 50, "")
	assert.NoError(t, err)
	assert.Len(t, rooms, 1)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestRoomRepo_ListRooms_InvalidCursor(t *testing.T) {
	repo, _ := newTestRoomRepo(t)
	_, _, err := repo.ListRooms(context.Background(), "not-a-cursor!!", 50, "")
	assert.Error(t, err)
}

func ptrStr(s string) *string { return &s }
