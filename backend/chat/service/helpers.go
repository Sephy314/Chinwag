package service

import (
	"encoding/json"
	"net/http"

	"github.com/Sephy314/chinwag/chat/domain"
	"github.com/Sephy314/chinwag/chat/structs"
	"github.com/Sephy314/chinwag/shared/errs"
)

var errNotAuthor = &errs.AppError{
	Status:  http.StatusForbidden,
	Message: "You are not the author of this message",
}

func toResponse(msg domain.ChatMessage, authorName string) *structs.MessageResponse {
	return &structs.MessageResponse{
		Id:          msg.Id.String(),
		RoomId:      msg.RoomId.String(),
		AuthorId:    msg.AuthorId.String(),
		AuthorName:  authorName,
		MessageType: msg.MessageType,
		Content:     msg.Content,
		CreatedAt:   msg.CreatedAt,
		UpdatedAt:   msg.UpdatedAt,
	}
}

type wsEvent struct {
	Type string      `json:"type"`
	Data interface{} `json:"data"`
}

func encodeEvent(eventType string, data interface{}) ([]byte, error) {
	return json.Marshal(wsEvent{Type: eventType, Data: data})
}
