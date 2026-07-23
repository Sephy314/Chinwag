package service

import (
	"context"
	"net/http"
	"time"

	"github.com/Sephy314/chinwag/chat/domain"
	"github.com/Sephy314/chinwag/chat/repo"
	"github.com/Sephy314/chinwag/chat/structs"
	"github.com/Sephy314/chinwag/conn/bridge"
	"github.com/Sephy314/chinwag/shared/errs"
	"github.com/google/uuid"
)

type BroadcastFunc func(roomId uuid.UUID, event []byte)

type ChatServiceInterface interface {
	CreateMessage(ctx context.Context, roomId uuid.UUID, req structs.CreateMessageRequest) (*structs.MessageResponse, error)
	GetMessage(ctx context.Context, messageId uuid.UUID) (*structs.MessageResponse, error)
	ListMessages(ctx context.Context, req structs.ListMessagesRequest) ([]structs.MessageResponse, *structs.CursorMeta, error)
	UpdateMessage(ctx context.Context, messageId uuid.UUID, userId uuid.UUID, req structs.UpdateMessageRequest) (*structs.MessageResponse, error)
	DeleteMessage(ctx context.Context, messageId uuid.UUID, userId uuid.UUID) error
}

type ChatService struct {
	repo      repo.ChatRepoInterface
	uow       repo.UnitOfWork
	user      bridge.UserProvider
	member    bridge.RoomMemberProvider
	broadcast BroadcastFunc
}

func NewChatService(chatRepo repo.ChatRepoInterface, uow repo.UnitOfWork, user bridge.UserProvider, member bridge.RoomMemberProvider, broadcast BroadcastFunc) *ChatService {
	return &ChatService{
		repo:      chatRepo,
		uow:       uow,
		user:      user,
		member:    member,
		broadcast: broadcast,
	}
}

func (s *ChatService) CreateMessage(ctx context.Context, roomId uuid.UUID, req structs.CreateMessageRequest) (*structs.MessageResponse, error) {
	authorId := ctx.Value("authorId").(uuid.UUID)

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

	id := uuid.Must(uuid.NewV7())
	now := time.Now()

	msg := domain.ChatMessage{
		Id:          id,
		RoomId:      roomId,
		AuthorId:    authorId,
		MessageType: req.MessageType,
		Content:     req.Content,
		CreatedAt:   now,
	}

	if s.uow == nil {
		err = s.repo.CreateMessage(ctx, msg)
	} else {
		err = s.uow.Do(ctx, func(txCtx context.Context, tx repo.Transaction) error {
			return tx.ChatRepo().CreateMessage(txCtx, msg)
		})
	}
	if err != nil {
		return nil, err
	}

	resp := toResponse(msg, user.Name)

	if s.broadcast != nil {
		event, _ := encodeEvent("new_message", resp)
		if event != nil {
			s.broadcast(roomId, event)
		}
	}

	return resp, nil
}

func (s *ChatService) GetMessage(ctx context.Context, messageId uuid.UUID) (*structs.MessageResponse, error) {
	msg, err := s.repo.GetMessageById(ctx, messageId)
	if err != nil {
		return nil, err
	}

	user, err := s.user.GetUser(ctx, msg.AuthorId.String())
	if err != nil {
		return nil, err
	}

	return toResponse(msg, user.Name), nil
}

func (s *ChatService) ListMessages(ctx context.Context, req structs.ListMessagesRequest) ([]structs.MessageResponse, *structs.CursorMeta, error) {
	roomId, err := uuid.Parse(req.RoomID)
	if err != nil {
		return nil, nil, err
	}

	msgs, meta, err := s.repo.ListMessagesByRoomId(ctx, roomId, req.Cursor, req.Limit)
	if err != nil {
		return nil, nil, err
	}

	result := make([]structs.MessageResponse, len(msgs))
	for i, m := range msgs {
		user, err := s.user.GetUser(ctx, m.AuthorId.String())
		authorName := ""
		if err == nil {
			authorName = user.Name
		}
		result[i] = *toResponse(m, authorName)
	}

	return result, meta, nil
}

func (s *ChatService) UpdateMessage(ctx context.Context, messageId uuid.UUID, userId uuid.UUID, req structs.UpdateMessageRequest) (*structs.MessageResponse, error) {
	msg, err := s.repo.GetMessageById(ctx, messageId)
	if err != nil {
		return nil, err
	}

	if msg.AuthorId != userId {
		return nil, errNotAuthor
	}

	if req.Content != nil {
		msg.Content = *req.Content
	}

	if s.uow == nil {
		err = s.repo.UpdateMessage(ctx, msg)
	} else {
		err = s.uow.Do(ctx, func(txCtx context.Context, tx repo.Transaction) error {
			return tx.ChatRepo().UpdateMessage(txCtx, msg)
		})
	}
	if err != nil {
		return nil, err
	}

	updated, err := s.repo.GetMessageById(ctx, messageId)
	if err != nil {
		return nil, err
	}

	user, err := s.user.GetUser(ctx, updated.AuthorId.String())
	if err != nil {
		return nil, err
	}

	resp := toResponse(updated, user.Name)

	if s.broadcast != nil {
		event, _ := encodeEvent("updated_message", resp)
		if event != nil {
			s.broadcast(msg.RoomId, event)
		}
	}

	return resp, nil
}

func (s *ChatService) DeleteMessage(ctx context.Context, messageId uuid.UUID, userId uuid.UUID) error {
	msg, err := s.repo.GetMessageById(ctx, messageId)
	if err != nil {
		return err
	}

	if msg.AuthorId != userId {
		return errNotAuthor
	}

	if s.uow == nil {
		err = s.repo.DeleteMessage(ctx, messageId)
	} else {
		err = s.uow.Do(ctx, func(txCtx context.Context, tx repo.Transaction) error {
			return tx.ChatRepo().DeleteMessage(txCtx, messageId)
		})
	}
	if err != nil {
		return err
	}

	if s.broadcast != nil {
		deletedEvent := struct {
			Id     string `json:"id"`
			RoomId string `json:"room_id"`
		}{
			Id:     messageId.String(),
			RoomId: msg.RoomId.String(),
		}
		event, _ := encodeEvent("deleted_message", deletedEvent)
		if event != nil {
			s.broadcast(msg.RoomId, event)
		}
	}

	return nil
}
