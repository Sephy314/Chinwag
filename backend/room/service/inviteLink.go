package service

import (
	"context"
	"encoding/json"
	"time"

	"github.com/Sephy314/chinwag/conn/bridge"
	"github.com/Sephy314/chinwag/conn/cache"
	"github.com/Sephy314/chinwag/room/repo"
	"github.com/Sephy314/chinwag/room/structs"
	"github.com/Sephy314/chinwag/shared/errs"
	"github.com/google/uuid"
)

const inviteKeyPrefix = "invite:"
const defaultInviteTTL = 24 * time.Hour

type inviteLinkData struct {
	RoomId    string `json:"room_id"`
	CreatedBy string `json:"created_by"`
	SingleUse bool   `json:"single_use"`
}

type InviteLinkServiceInterface interface {
	CreateInviteLink(ctx context.Context, roomId uuid.UUID, createdBy uuid.UUID, req structs.CreateInviteLinkRequest) (*structs.InviteLinkResponse, error)
	JoinByInviteLink(ctx context.Context, token string, userId uuid.UUID) error
}

type InviteLinkService struct {
	cache         cache.Cache
	roomMemberSvc RoomMemberServiceInterface
	userProvider  bridge.UserProvider
	roomRepo      repo.RoomRepoInterface
}

func NewInviteLinkService(
	c cache.Cache,
	roomMemberSvc RoomMemberServiceInterface,
	userProvider bridge.UserProvider,
	roomRepo repo.RoomRepoInterface,
) *InviteLinkService {
	return &InviteLinkService{
		cache:         c,
		roomMemberSvc: roomMemberSvc,
		userProvider:  userProvider,
		roomRepo:      roomRepo,
	}
}

func (s *InviteLinkService) CreateInviteLink(ctx context.Context, roomId uuid.UUID, createdBy uuid.UUID, req structs.CreateInviteLinkRequest) (*structs.InviteLinkResponse, error) {
	room, err := s.roomRepo.GetRoomById(ctx, roomId)
	if err != nil {
		return nil, err
	}

	if room.PoppedAt != nil {
		return nil, errs.ErrRoomPopped
	}

	ok, err := s.roomMemberSvc.HasManagerPermission(ctx, createdBy, roomId)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, &errs.AppError{
			Status:  403,
			Message: "Admin permission is required",
		}
	}

	ttl := defaultInviteTTL
	if req.TTLHours != nil && *req.TTLHours > 0 {
		ttl = time.Duration(*req.TTLHours) * time.Hour
	}

	token := uuid.New().String()

	data := inviteLinkData{
		RoomId:    roomId.String(),
		CreatedBy: createdBy.String(),
		SingleUse: req.SingleUse != nil && *req.SingleUse,
	}

	jsonData, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}

	key := inviteKeyPrefix + token
	if err := s.cache.Set(ctx, key, string(jsonData), ttl); err != nil {
		return nil, err
	}

	return &structs.InviteLinkResponse{
		Token:     token,
		RoomId:    roomId.String(),
		ExpiresAt: time.Now().Add(ttl),
	}, nil
}

func (s *InviteLinkService) JoinByInviteLink(ctx context.Context, token string, userId uuid.UUID) error {
	key := inviteKeyPrefix + token
	jsonData, err := s.cache.Get(ctx, key)
	if err != nil {
		return errs.ErrInviteNotFound
	}

	var data inviteLinkData
	if err := json.Unmarshal([]byte(jsonData), &data); err != nil {
		return errs.ErrInviteNotFound
	}

	roomId, err := uuid.Parse(data.RoomId)
	if err != nil {
		return errs.ErrInviteNotFound
	}

	room, err := s.roomRepo.GetRoomById(ctx, roomId)
	if err != nil {
		return err
	}

	if room.PoppedAt != nil {
		return errs.ErrRoomPopped
	}

	user, err := s.userProvider.GetUser(ctx, userId.String())
	if err != nil || user == nil {
		return errs.ErrUserDeleted
	}

	existing, err := s.roomMemberSvc.GetUserByRoomIdAndUserId(ctx, userId, roomId)
	if err == nil && existing != nil {
		return errs.ErrAlreadyMember
	}

	member := structs.RoomUser{
		UserId: userId,
		RoomId: roomId,
	}

	if err := s.roomMemberSvc.InviteUser(ctx, member); err != nil {
		return err
	}

	if data.SingleUse {
		_ = s.cache.Delete(ctx, key)
	}

	return nil
}

//
//func inviteKey(token string) string {
//	return fmt.Sprintf("%s%s", inviteKeyPrefix, token)
//}
