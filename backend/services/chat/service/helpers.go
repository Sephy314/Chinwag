package service

import (
	"net/http"

	"github.com/Sephy314/chinwag/backend/services/chat/domain"
	"github.com/Sephy314/chinwag/backend/services/chat/structs"
	"github.com/Sephy314/chinwag/backend/services/chat/shared/errs"
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
