package repo

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"time"

	"github.com/Sephy314/chinwag/chat/domain"
	"github.com/Sephy314/chinwag/chat/structs"
	"github.com/Sephy314/chinwag/shared/errs"
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

const defaultLimit = 50
const maxLimit = 200

type ChatRepoInterface interface {
	CreateMessage(ctx context.Context, msg domain.ChatMessage) error
	GetMessageById(ctx context.Context, id uuid.UUID) (domain.ChatMessage, error)
	ListMessagesByRoomId(ctx context.Context, roomId uuid.UUID, cursorStr string, limit int) ([]domain.ChatMessage, *structs.CursorMeta, error)
	UpdateMessage(ctx context.Context, msg domain.ChatMessage) error
	DeleteMessage(ctx context.Context, id uuid.UUID) error
}

type ChatRepo struct {
	db sqlx.ExtContext
}

func NewChatRepo(db sqlx.ExtContext) *ChatRepo {
	return &ChatRepo{db: db}
}

func (r *ChatRepo) CreateMessage(ctx context.Context, msg domain.ChatMessage) error {
	_, err := r.db.ExecContext(
		ctx,
		`INSERT INTO chat_messages (id, room_id, author_id, message_type, content)
		 VALUES ($1, $2, $3, $4, $5)`,
		msg.Id, msg.RoomId, msg.AuthorId, msg.MessageType, msg.Content,
	)
	return err
}

func (r *ChatRepo) GetMessageById(ctx context.Context, id uuid.UUID) (domain.ChatMessage, error) {
	var msg domain.ChatMessage
	err := sqlx.GetContext(
		ctx, r.db, &msg,
		`SELECT id, room_id, author_id, message_type, content, created_at, updated_at, deleted_at
		 FROM chat_messages
		 WHERE id = $1 AND deleted_at IS NULL`,
		id,
	)
	return msg, err
}

func (r *ChatRepo) ListMessagesByRoomId(ctx context.Context, roomId uuid.UUID, cursorStr string, limit int) ([]domain.ChatMessage, *structs.CursorMeta, error) {
	if limit <= 0 || limit > maxLimit {
		limit = defaultLimit
	}

	var msgs []domain.ChatMessage
	var err error

	fetchLimit := limit + 1

	if cursorStr == "" {
		err = sqlx.SelectContext(
			ctx, r.db, &msgs,
			`SELECT id, room_id, author_id, message_type, content, created_at, updated_at, deleted_at
			 FROM chat_messages
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
			`SELECT id, room_id, author_id, message_type, content, created_at, updated_at, deleted_at
			 FROM chat_messages
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

func (r *ChatRepo) UpdateMessage(ctx context.Context, msg domain.ChatMessage) error {
	res, err := r.db.ExecContext(
		ctx,
		`UPDATE chat_messages SET content = $1, updated_at = NOW()
		 WHERE id = $2 AND deleted_at IS NULL`,
		msg.Content, msg.Id,
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

func (r *ChatRepo) DeleteMessage(ctx context.Context, id uuid.UUID) error {
	res, err := r.db.ExecContext(
		ctx,
		`UPDATE chat_messages SET deleted_at = NOW()
		 WHERE id = $1 AND deleted_at IS NULL`,
		id,
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
