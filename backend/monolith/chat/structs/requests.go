package structs

import "github.com/Sephy314/chinwag/backend/monolith/chat/domain"

type CreateMessageRequest struct {
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
