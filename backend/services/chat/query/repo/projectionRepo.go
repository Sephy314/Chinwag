package repo

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"strings"
	"time"

	"github.com/Sephy314/chinwag/backend/services/chat/query/domain"
	"github.com/Sephy314/chinwag/backend/services/chat/query/shared/errs"
	"github.com/Sephy314/chinwag/backend/services/chat/query/structs"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

type cursor struct {
	CreatedAt time.Time `json:"created_at"`
	Id        uuid.UUID `json:"id"`
}

func encodeCursor(createdAt time.Time, id uuid.UUID) string {
	c := cursor{CreatedAt: createdAt, Id: id}
	b, _ := json.Marshal(c)
	return base64.URLEncoding.EncodeToString(b)
}

func decodeCursor(s string) (cursor, error) {
	c := cursor{}
	b, err := base64.URLEncoding.DecodeString(s)
	if err != nil {
		return c, err
	}
	err = json.Unmarshal(b, &c)
	return c, err
}

// rebind converts "?" placeholders to the pgx ($N) positional style.
func rebind(q string) string {
	var b strings.Builder
	n := 0
	for _, r := range q {
		if r == '?' {
			n++
			b.WriteString("$" + itoa(n))
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}

const defaultLimit = 50
const maxLimit = 200

type ProjectionRepoInterface interface {
	Upsert(ctx context.Context, msg domain.MessageProjection) error
	GetById(ctx context.Context, id uuid.UUID) (domain.MessageProjection, error)
	ListByRoomId(ctx context.Context, roomId uuid.UUID, cursorStr string, limit int) ([]domain.MessageProjection, *structs.CursorMeta, error)
	ListAfterByRoomId(ctx context.Context, roomId uuid.UUID, afterCursor string, limit int) ([]domain.MessageProjection, error)
	UpdateContent(ctx context.Context, id uuid.UUID, content string, updatedAt time.Time) error
	SoftDelete(ctx context.Context, id uuid.UUID, deletedAt time.Time) error
	AdminListMessages(ctx context.Context, cursorStr string, limit int, roomID, authorID *uuid.UUID, search string) ([]domain.MessageProjection, *structs.CursorMeta, error)
	AdminGetMessageIncludingDeleted(ctx context.Context, id uuid.UUID) (domain.MessageProjection, error)
	AdminCountMessages(ctx context.Context) (int, error)
}

type ProjectionRepo struct {
	db sqlx.ExtContext
}

func NewProjectionRepo(db sqlx.ExtContext) *ProjectionRepo {
	return &ProjectionRepo{db: db}
}

func (r *ProjectionRepo) Upsert(ctx context.Context, msg domain.MessageProjection) error {
	_, err := r.db.ExecContext(
		ctx,
		`INSERT INTO message_projections (id, room_id, author_id, author_name, message_type, content, created_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)
		 ON CONFLICT (id) DO UPDATE SET
		   content = EXCLUDED.content,
		   updated_at = NOW()`,
		msg.Id, msg.RoomId, msg.AuthorId, msg.AuthorName, msg.MessageType, msg.Content, msg.CreatedAt,
	)
	return err
}

func (r *ProjectionRepo) GetById(ctx context.Context, id uuid.UUID) (domain.MessageProjection, error) {
	var msg domain.MessageProjection
	err := sqlx.GetContext(
		ctx, r.db, &msg,
		`SELECT id, room_id, author_id, author_name, message_type, content, created_at, updated_at, deleted_at
		 FROM message_projections
		 WHERE id = $1 AND deleted_at IS NULL`,
		id,
	)
	return msg, err
}

func (r *ProjectionRepo) ListByRoomId(ctx context.Context, roomId uuid.UUID, cursorStr string, limit int) ([]domain.MessageProjection, *structs.CursorMeta, error) {
	if limit <= 0 || limit > maxLimit {
		limit = defaultLimit
	}

	var msgs []domain.MessageProjection
	var err error

	fetchLimit := limit + 1

	if cursorStr == "" {
		err = sqlx.SelectContext(
			ctx, r.db, &msgs,
			`SELECT id, room_id, author_id, author_name, message_type, content, created_at, updated_at, deleted_at
			 FROM message_projections
			 WHERE room_id = $1 AND deleted_at IS NULL
			 ORDER BY created_at DESC, id DESC
			 LIMIT $2`,
			roomId, fetchLimit,
		)
	} else {
		c, cerr := decodeCursor(cursorStr)
		if cerr != nil {
			return nil, nil, cerr
		}
		err = sqlx.SelectContext(
			ctx, r.db, &msgs,
			`SELECT id, room_id, author_id, author_name, message_type, content, created_at, updated_at, deleted_at
			 FROM message_projections
			 WHERE room_id = $1 AND deleted_at IS NULL
			   AND (created_at, id) < ($2, $3)
			 ORDER BY created_at DESC, id DESC
			 LIMIT $4`,
			roomId, c.CreatedAt, c.Id, fetchLimit,
		)
	}
	if err != nil {
		return nil, nil, err
	}

	hasMore := len(msgs) > limit
	if hasMore {
		msgs = msgs[:limit]
	}

	var meta *structs.CursorMeta
	if hasMore && len(msgs) > 0 {
		last := msgs[len(msgs)-1]
		meta = &structs.CursorMeta{
			NextCursor: encodeCursor(last.CreatedAt, last.Id),
			HasMore:    true,
		}
	}

	return msgs, meta, nil
}

// ListAfterByRoomId returns messages strictly newer than the given cursor in
// ascending (oldest first) order. It is used to reconcile the gap after a
// WebSocket reconnect: the client passes the newest message it already has as
// the after cursor and only the missed messages are fetched.
func (r *ProjectionRepo) ListAfterByRoomId(ctx context.Context, roomId uuid.UUID, afterCursor string, limit int) ([]domain.MessageProjection, error) {
	if limit <= 0 || limit > maxLimit {
		limit = maxLimit
	}

	c, err := decodeCursor(afterCursor)
	if err != nil {
		return nil, err
	}

	var msgs []domain.MessageProjection
	err = sqlx.SelectContext(
		ctx, r.db, &msgs,
		`SELECT id, room_id, author_id, author_name, message_type, content, created_at, updated_at, deleted_at
		 FROM message_projections
		 WHERE room_id = $1 AND deleted_at IS NULL
		   AND (created_at, id) > ($2, $3)
		 ORDER BY created_at ASC, id ASC
		 LIMIT $4`,
		roomId, c.CreatedAt, c.Id, limit,
	)
	if err != nil {
		return nil, err
	}

	return msgs, nil
}

func (r *ProjectionRepo) UpdateContent(ctx context.Context, id uuid.UUID, content string, updatedAt time.Time) error {
	res, err := r.db.ExecContext(
		ctx,
		`UPDATE message_projections SET content = $1, updated_at = $2
		 WHERE id = $3 AND deleted_at IS NULL`,
		content, updatedAt, id,
	)
	if err != nil {
		return err
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return errs.ErrNotFound
	}
	return nil
}

func (r *ProjectionRepo) SoftDelete(ctx context.Context, id uuid.UUID, deletedAt time.Time) error {
	res, err := r.db.ExecContext(
		ctx,
		`UPDATE message_projections SET deleted_at = $1 WHERE id = $2 AND deleted_at IS NULL`,
		deletedAt, id,
	)
	if err != nil {
		return err
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return errs.ErrNotFound
	}
	return nil
}

// AdminListMessages lists messages across all rooms with optional filters
// (room, author, content search) and cursor pagination. Admin only.
func (r *ProjectionRepo) AdminListMessages(ctx context.Context, cursorStr string, limit int, roomID, authorID *uuid.UUID, search string) ([]domain.MessageProjection, *structs.CursorMeta, error) {
	if limit <= 0 || limit > maxLimit {
		limit = defaultLimit
	}

	where := []string{"deleted_at IS NULL"}
	args := []any{}
	arg := func(v any) string {
		args = append(args, v)
		return "?"
	}
	if roomID != nil {
		where = append(where, "room_id = "+arg(*roomID))
	}
	if authorID != nil {
		where = append(where, "author_id = "+arg(*authorID))
	}
	if search != "" {
		where = append(where, "content ILIKE "+arg("%"+search+"%"))
	}

	query := `SELECT id, room_id, author_id, author_name, message_type, content, created_at, updated_at, deleted_at
	          FROM message_projections WHERE ` + strings.Join(where, " AND ")

	if cursorStr == "" {
		query += ` ORDER BY created_at DESC, id DESC LIMIT ?`
	} else {
		c, cerr := decodeCursor(cursorStr)
		if cerr != nil {
			return nil, nil, cerr
		}
		query += ` AND (created_at, id) < (?, ?) ORDER BY created_at DESC, id DESC LIMIT ?`
		args = append(args, c.CreatedAt, c.Id)
	}
	args = append(args, limit+1)
	query = rebind(query)

	var msgs []domain.MessageProjection
	if err := sqlx.SelectContext(ctx, r.db, &msgs, query, args...); err != nil {
		return nil, nil, err
	}

	hasMore := len(msgs) > limit
	if hasMore {
		msgs = msgs[:limit]
	}
	var meta *structs.CursorMeta
	if hasMore && len(msgs) > 0 {
		last := msgs[len(msgs)-1]
		meta = &structs.CursorMeta{
			NextCursor: encodeCursor(last.CreatedAt, last.Id),
			HasMore:    true,
		}
	}
	return msgs, meta, nil
}

// AdminGetMessageIncludingDeleted returns a message even if it has been
// soft-deleted. Admin only.
func (r *ProjectionRepo) AdminGetMessageIncludingDeleted(ctx context.Context, id uuid.UUID) (domain.MessageProjection, error) {
	var msg domain.MessageProjection
	err := sqlx.GetContext(
		ctx, r.db, &msg,
		`SELECT id, room_id, author_id, author_name, message_type, content, created_at, updated_at, deleted_at
		 FROM message_projections
		 WHERE id = $1`,
		id,
	)
	return msg, err
}

// AdminCountMessages returns the number of active (non-deleted) messages.
func (r *ProjectionRepo) AdminCountMessages(ctx context.Context) (int, error) {
	var n int
	err := sqlx.GetContext(ctx, r.db, &n, `SELECT COUNT(*) FROM message_projections WHERE deleted_at IS NULL`)
	return n, err
}
