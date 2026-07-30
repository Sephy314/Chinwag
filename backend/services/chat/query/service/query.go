package service

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/Sephy314/chinwag/backend/services/chat/query/domain"
	"github.com/Sephy314/chinwag/backend/services/chat/query/repo"
	"github.com/Sephy314/chinwag/backend/services/chat/query/shared/cache"
	"github.com/Sephy314/chinwag/backend/services/chat/query/shared/errs"
	"github.com/Sephy314/chinwag/backend/services/chat/query/structs"
	"github.com/google/uuid"
)

const (
	cachePrefix = "chat:"
	cacheTTL    = 6 * time.Hour
)

type QueryServiceInterface interface {
	GetMessage(ctx context.Context, messageId uuid.UUID, userId uuid.UUID) (*structs.MessageResponse, error)
	ListMessages(ctx context.Context, req structs.ListMessagesRequest) ([]structs.MessageResponse, *structs.CursorMeta, error)
}

type RoomMemberProvider interface {
	GetMembersByRoomId(ctx context.Context, roomId string) ([]RoomMemberInfo, error)
}

type RoomMemberInfo struct {
	RoomId   string
	UserId   string
	Role     int
	JoinedAt string
	LeftAt   *string
}

type QueryService struct {
	repo   repo.ProjectionRepoInterface
	member RoomMemberProvider
	cache  cache.Cache
}

func NewQueryService(projectionRepo repo.ProjectionRepoInterface, member RoomMemberProvider, cache cache.Cache) *QueryService {
	return &QueryService{
		repo:   projectionRepo,
		member: member,
		cache:  cache,
	}
}

func (s *QueryService) GetMessage(ctx context.Context, messageId uuid.UUID, userId uuid.UUID) (*structs.MessageResponse, error) {
	cacheKey := cachePrefix + messageId.String()

	cached, err := s.cache.Get(ctx, cacheKey)
	if err == nil && cached != "" {
		var msg structs.MessageResponse
		if json.Unmarshal([]byte(cached), &msg) == nil {
			members, err := s.member.GetMembersByRoomId(ctx, msg.RoomId)
			if err != nil {
				return nil, err
			}
			isMember := false
			for _, m := range members {
				if m.UserId == userId.String() {
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
			return &msg, nil
		}
	}

	msg, err := s.repo.GetById(ctx, messageId)
	if err != nil {
		return nil, err
	}

	members, err := s.member.GetMembersByRoomId(ctx, msg.RoomId.String())
	if err != nil {
		return nil, err
	}
	isMember := false
	for _, m := range members {
		if m.UserId == userId.String() {
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

	resp := toResponse(msg)

	if b, err := json.Marshal(resp); err == nil {
		s.cache.Set(ctx, cacheKey, string(b), cacheTTL)
	}

	return resp, nil
}

func (s *QueryService) ListMessages(ctx context.Context, req structs.ListMessagesRequest) ([]structs.MessageResponse, *structs.CursorMeta, error) {
	roomId, err := uuid.Parse(req.RoomID)
	if err != nil {
		return nil, nil, fmt.Errorf("invalid room id: %w", err)
	}

	userId := ctx.Value("userId").(uuid.UUID)
	members, err := s.member.GetMembersByRoomId(ctx, roomId.String())
	if err != nil {
		return nil, nil, err
	}
	isMember := false
	for _, m := range members {
		if m.UserId == userId.String() {
			isMember = true
			break
		}
	}
	if !isMember {
		return nil, nil, &errs.AppError{
			Status:  http.StatusForbidden,
			Message: "You are not a member of this room",
		}
	}

	msgs, meta, err := s.repo.ListByRoomId(ctx, roomId, req.Cursor, req.Limit)
	if err != nil {
		return nil, nil, err
	}

	result := make([]structs.MessageResponse, len(msgs))
	for i, m := range msgs {
		resp := toResponse(m)
		result[i] = *resp

		cacheKey := cachePrefix + m.Id.String()
		if b, err := json.Marshal(resp); err == nil {
			s.cache.Set(ctx, cacheKey, string(b), cacheTTL)
		}
	}

	return result, meta, nil
}

func toResponse(msg domain.MessageProjection) *structs.MessageResponse {
	return &structs.MessageResponse{
		Id:          msg.Id.String(),
		RoomId:      msg.RoomId.String(),
		AuthorId:    msg.AuthorId.String(),
		AuthorName:  msg.AuthorName,
		MessageType: msg.MessageType,
		Content:     msg.Content,
		CreatedAt:   msg.CreatedAt,
		UpdatedAt:   msg.UpdatedAt,
	}
}
