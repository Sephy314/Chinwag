package structs

import (
	"time"

	"github.com/Sephy314/chinwag/chat/domain"
)

type MessageResponse struct {
	Id          string             `json:"id"`
	RoomId      string             `json:"room_id"`
	AuthorId    string             `json:"author_id"`
	AuthorName  string             `json:"author_name"`
	MessageType domain.MessageType `json:"message_type"`
	Content     string             `json:"content"`
	CreatedAt   time.Time          `json:"created_at"`
	UpdatedAt   *time.Time         `json:"updated_at,omitempty"`
}

type CursorMeta struct {
	NextCursor string `json:"next_cursor,omitempty"`
	HasMore    bool   `json:"has_more"`
}
