package service

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
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

	if fields, err := s.cache.HGetAll(ctx, cacheKey); err == nil && len(fields) > 0 {
		if msg, err := hashToResponse(fields); err == nil {
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
			return msg, nil
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
	_ = s.cache.HSet(ctx, cacheKey, responseToHash(resp), cacheTTL)

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

	// Reconnect resync path: fetch only messages strictly newer than the
	// client's newest known message (ascending) using the existing cursor.
	if req.After != "" {
		msgs, err := s.repo.ListAfterByRoomId(ctx, roomId, req.After, req.Limit)
		if err != nil {
			return nil, nil, err
		}

		result := make([]structs.MessageResponse, len(msgs))
		for i, m := range msgs {
			resp := toResponse(m)
			result[i] = *resp

			cacheKey := cachePrefix + m.Id.String()
			s.cache.HSet(ctx, cacheKey, responseToHash(resp), cacheTTL)
		}

		return result, nil, nil
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
		_ = s.cache.HSet(ctx, cacheKey, responseToHash(resp), cacheTTL)
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

func responseToHash(msg *structs.MessageResponse) map[string]string {
	fields := map[string]string{
		"id":           msg.Id,
		"room_id":      msg.RoomId,
		"author_id":    msg.AuthorId,
		"author_name":  msg.AuthorName,
		"message_type": strconv.FormatInt(int64(msg.MessageType), 10),
		"content":      msg.Content,
		"created_at":   msg.CreatedAt.Format(time.RFC3339Nano),
	}
	if msg.UpdatedAt != nil {
		fields["updated_at"] = msg.UpdatedAt.Format(time.RFC3339Nano)
	}
	return fields
}

func hashToResponse(fields map[string]string) (*structs.MessageResponse, error) {
	if fields["id"] == "" {
		return nil, errors.New("cache entry missing id")
	}
	messageType, err := strconv.ParseInt(fields["message_type"], 10, 16)
	if err != nil {
		return nil, err
	}
	createdAt, err := time.Parse(time.RFC3339Nano, fields["created_at"])
	if err != nil {
		return nil, err
	}

	resp := &structs.MessageResponse{
		Id:          fields["id"],
		RoomId:      fields["room_id"],
		AuthorId:    fields["author_id"],
		AuthorName:  fields["author_name"],
		MessageType: int16(messageType),
		Content:     fields["content"],
		CreatedAt:   createdAt,
	}
	if updatedStr, ok := fields["updated_at"]; ok && updatedStr != "" {
		updatedAt, err := time.Parse(time.RFC3339Nano, updatedStr)
		if err != nil {
			return nil, err
		}
		resp.UpdatedAt = &updatedAt
	}
	return resp, nil
}
