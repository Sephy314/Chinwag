package structs

import (
	"github.com/Sephy314/chinwag/backend/services/chat/command/domain"
	"github.com/google/uuid"
)

type CreateMessageRequest struct {
	Id          uuid.UUID          `json:"id"`
	MessageType domain.MessageType `json:"message_type"`
	Content     string             `json:"content"`
}

type UpdateMessageRequest struct {
	Content *string `json:"content,omitempty"`
}

type ListMessagesRequest struct {
	RoomID string
	Cursor string `query:"cursor"`
	Limit  int    `query:"limit"`
}
