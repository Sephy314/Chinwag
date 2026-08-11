package service

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/Sephy314/chinwag/backend/services/chat/command/domain"
	"github.com/Sephy314/chinwag/backend/services/chat/command/repo"
	"github.com/Sephy314/chinwag/backend/services/chat/command/shared/errs"
	"github.com/Sephy314/chinwag/backend/services/chat/command/structs"
	"github.com/google/uuid"
)

type ChatServiceInterface interface {
	CreateMessage(ctx context.Context, roomId uuid.UUID, req structs.CreateMessageRequest) (*structs.MessageResponse, error)
	UpdateMessage(ctx context.Context, messageId uuid.UUID, userId uuid.UUID, req structs.UpdateMessageRequest) (*structs.MessageResponse, error)
	DeleteMessage(ctx context.Context, messageId uuid.UUID, userId uuid.UUID) error
	AdminDeleteMessage(ctx context.Context, messageId uuid.UUID) error
}

type ChatService struct {
	repo   repo.ChatRepoInterface
	uow    repo.UnitOfWork
	user   UserProvider
	member RoomMemberProvider
}

func NewChatService(chatRepo repo.ChatRepoInterface, uow repo.UnitOfWork, user UserProvider, member RoomMemberProvider) *ChatService {
	return &ChatService{
		repo:   chatRepo,
		uow:    uow,
		user:   user,
		member: member,
	}
}

func (s *ChatService) CreateMessage(ctx context.Context, roomId uuid.UUID, req structs.CreateMessageRequest) (*structs.MessageResponse, error) {
	authorId := ctx.Value("authorId").(uuid.UUID)

	room, err := s.member.GetRoomById(ctx, roomId.String())
	if err != nil {
		return nil, err
	}
	if room.PoppedAt != nil {
		return nil, &errs.AppError{
			Status:  http.StatusForbidden,
			Message: "This room has been popped and is now read-only",
		}
	}

	members, err := s.member.GetMembersByRoomId(ctx, roomId.String())
	if err != nil {
		return nil, err
	}
	isMember := false
	for _, m := range members {
		if m.UserId == authorId.String() {
			isMember = true
			break
		}
	}
	if !isMember {
		return nil, &errs.AppError{
			Status:  http.StatusForbidden,
			Message: "You are not a member of this room",
		}
	}

	user, err := s.user.GetUser(ctx, authorId.String())
	if err != nil {
		return nil, err
	}

	now := time.Now()

	msg := domain.ChatMessage{
		Id:          req.Id,
		RoomId:      roomId,
		AuthorId:    authorId,
		MessageType: req.MessageType,
		Content:     req.Content,
		CreatedAt:   now,
	}

	resp := toResponse(msg, user.Name)

	evPayload, err := json.Marshal(map[string]any{
		"type": "new_message",
		"data": resp,
	})
	if err != nil {
		return nil, fmt.Errorf("marshal event: %w", err)
	}

	err = s.uow.Do(ctx, func(txCtx context.Context, tx repo.Transaction) error {
		if err := tx.ChatRepo().CreateMessage(txCtx, msg); err != nil {
			return err
		}
		return tx.OutboxRepo().Insert(txCtx, repo.OutboxEvent{
			Id:        uuid.Must(uuid.NewV7()),
			EventType: "message_created",
			Subject:   fmt.Sprintf("chat.room.%s", roomId.String()),
			Payload:   evPayload,
			RoomId:    roomId,
			CreatedAt: now,
		})
	})
	if err != nil {
		return nil, err
	}

	return resp, nil
}

func (s *ChatService) UpdateMessage(ctx context.Context, messageId uuid.UUID, userId uuid.UUID, req structs.UpdateMessageRequest) (*structs.MessageResponse, error) {
	msg, err := s.repo.GetMessageById(ctx, messageId)
	if err != nil {
		return nil, err
	}

	room, err := s.member.GetRoomById(ctx, msg.RoomId.String())
	if err != nil {
		return nil, err
	}
	if room.PoppedAt != nil {
		return nil, &errs.AppError{
			Status:  http.StatusForbidden,
			Message: "This room has been popped and is now read-only",
		}
	}

	if msg.AuthorId != userId {
		return nil, errNotAuthor
	}

	if req.Content != nil {
		msg.Content = *req.Content
	}

	now := time.Now()
	updated := msg
	updated.UpdatedAt = &now

	user, err := s.user.GetUser(ctx, updated.AuthorId.String())
	if err != nil {
		return nil, err
	}

	resp := toResponse(updated, user.Name)

	evPayload, err := json.Marshal(map[string]any{
		"type": "updated_message",
		"data": resp,
	})
	if err != nil {
		return nil, fmt.Errorf("marshal event: %w", err)
	}

	err = s.uow.Do(ctx, func(txCtx context.Context, tx repo.Transaction) error {
		if err := tx.ChatRepo().UpdateMessage(txCtx, msg); err != nil {
			return err
		}
		return tx.OutboxRepo().Insert(txCtx, repo.OutboxEvent{
			Id:        uuid.Must(uuid.NewV7()),
			EventType: "message_updated",
			Subject:   fmt.Sprintf("chat.room.%s", msg.RoomId.String()),
			Payload:   evPayload,
			RoomId:    msg.RoomId,
			CreatedAt: now,
		})
	})
	if err != nil {
		return nil, err
	}

	return resp, nil
}

func (s *ChatService) DeleteMessage(ctx context.Context, messageId uuid.UUID, userId uuid.UUID) error {
	msg, err := s.repo.GetMessageById(ctx, messageId)
	if err != nil {
		return err
	}

	room, err := s.member.GetRoomById(ctx, msg.RoomId.String())
	if err != nil {
		return err
	}
	if room.PoppedAt != nil {
		return &errs.AppError{
			Status:  http.StatusForbidden,
			Message: "This room has been popped and is now read-only",
		}
	}

	if msg.AuthorId != userId {
		return errNotAuthor
	}

	deletedEvent := struct {
		Id     string `json:"id"`
		RoomId string `json:"room_id"`
	}{
		Id:     messageId.String(),
		RoomId: msg.RoomId.String(),
	}

	evPayload, err := json.Marshal(map[string]any{
		"type": "deleted_message",
		"data": deletedEvent,
	})
	if err != nil {
		return fmt.Errorf("marshal event: %w", err)
	}

	err = s.uow.Do(ctx, func(txCtx context.Context, tx repo.Transaction) error {
		if err := tx.ChatRepo().DeleteMessage(txCtx, messageId); err != nil {
			return err
		}
		return tx.OutboxRepo().Insert(txCtx, repo.OutboxEvent{
			Id:        uuid.Must(uuid.NewV7()),
			EventType: "message_deleted",
			Subject:   fmt.Sprintf("chat.room.%s", msg.RoomId.String()),
			Payload:   evPayload,
			RoomId:    msg.RoomId,
			CreatedAt: time.Now(),
		})
	})
	if err != nil {
		return err
	}

	return nil
}

// AdminDeleteMessage soft-deletes a message on behalf of an admin. Unlike the
// user-facing DeleteMessage it skips both the author check and the popped-room
// read-only guard. The deletion is still propagated to the projection via the
// outbox so the query side stays consistent.
func (s *ChatService) AdminDeleteMessage(ctx context.Context, messageId uuid.UUID) error {
	msg, err := s.repo.GetMessageById(ctx, messageId)
	if err != nil {
		return err
	}

	deletedEvent := struct {
		Id     string `json:"id"`
		RoomId string `json:"room_id"`
	}{
		Id:     messageId.String(),
		RoomId: msg.RoomId.String(),
	}

	evPayload, err := json.Marshal(map[string]any{
		"type": "deleted_message",
		"data": deletedEvent,
	})
	if err != nil {
		return fmt.Errorf("marshal event: %w", err)
	}

	err = s.uow.Do(ctx, func(txCtx context.Context, tx repo.Transaction) error {
		if err := tx.ChatRepo().DeleteMessage(txCtx, messageId); err != nil {
			return err
		}
		return tx.OutboxRepo().Insert(txCtx, repo.OutboxEvent{
			Id:        uuid.Must(uuid.NewV7()),
			EventType: "message_deleted",
			Subject:   fmt.Sprintf("chat.room.%s", msg.RoomId.String()),
			Payload:   evPayload,
			RoomId:    msg.RoomId,
			CreatedAt: time.Now(),
		})
	})
	if err != nil {
		return err
	}

	return nil
}
