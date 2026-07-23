package domain

import (
	"time"

	"github.com/google/uuid"
)

type ChatMessage struct {
	Id          uuid.UUID   `db:"id" json:"id"`
	RoomId      uuid.UUID   `db:"room_id" json:"room_id"`
	AuthorId    uuid.UUID   `db:"author_id" json:"author_id"`
	MessageType MessageType `db:"message_type" json:"message_type"`
	Content     string      `db:"content" json:"content"`
	CreatedAt   time.Time   `db:"created_at" json:"created_at"`
	UpdatedAt   *time.Time  `db:"updated_at" json:"updated_at,omitempty"`
	DeletedAt   *time.Time  `db:"deleted_at" json:"-"`
}
