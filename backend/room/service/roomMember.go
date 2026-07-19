package service

import (
	"context"
	"time"

	"github.com/Sephy314/chinwag/room/domain"
	"github.com/Sephy314/chinwag/room/repo"
	"github.com/Sephy314/chinwag/room/structs"
	"github.com/Sephy314/chinwag/shared/errs"
	"github.com/Sephy314/chinwag/shared/patch"
	"github.com/google/uuid"
)

func (s *RoomMemberService) isRoomPopped(ctx context.Context, roomId uuid.UUID) (bool, error) {
	room, err := s.roomRepo.GetRoomById(ctx, roomId)
	if err != nil {
		return false, err
	}
	return room.PoppedAt != nil, nil
}

type RoomMemberServiceInterface interface {
	InviteUser(ctx context.Context, member structs.RoomUser) error
	KickUser(ctx context.Context, member structs.RoomUser) error
	GetUserByRoomId(ctx context.Context, roomId uuid.UUID) ([]domain.RoomMember, error)
	GetRoomsByUserId(ctx context.Context, userId uuid.UUID) ([]domain.Room, error)
	GetUserByRoomIdAndUserId(ctx context.Context, userId, roomId uuid.UUID) (*domain.RoomMember, error)
	UpdateRoomMember(ctx context.Context, userId, roomId uuid.UUID, req structs.UpdateRoomMemberRequest) (*domain.RoomMember, error)
	SetUserRole(ctx context.Context, userId uuid.UUID, roomId uuid.UUID, role domain.Role) error
	GetUserRole(ctx context.Context, userId uuid.UUID, roomId uuid.UUID) (*domain.Role, error)
	HasManagerPermission(ctx context.Context, userId uuid.UUID, roomId uuid.UUID) (bool, error)
}

type RoomMemberService struct {
	repo     repo.RoomMemberRepoInterface
	roomRepo repo.RoomRepoInterface
	uow      repo.UnitOfWork
}

func (s *RoomMemberService) GetRoomsByUserId(ctx context.Context, userId uuid.UUID) ([]domain.Room, error) {
	return s.repo.GetRoomsByUserId(ctx, userId)
}

func (s *RoomMemberService) UpdateRoomMember(ctx context.Context, userId, roomId uuid.UUID, req structs.UpdateRoomMemberRequest) (*domain.RoomMember, error) {
	popped, err := s.isRoomPopped(ctx, roomId)
	if err != nil {
		return nil, err
	}
	if popped {
		return nil, errs.ErrRoomPopped
	}

	member, err := s.repo.GetMemberByRoomIdAndMemberId(ctx, roomId, userId)
	if err != nil {
		return nil, err
	}

	_, err = patch.Patch(&member, req,
		patch.WithIgnore("RoomId", "UserId", "JoinedAt", "LeftAt"),
	)
	if err != nil {
		return nil, err
	}

	if s.uow == nil {
		err = s.repo.UpdateMember(ctx, member)
		if err != nil {
			return nil, err
		}
		return &member, nil
	}

	err = s.uow.Do(ctx, func(txCtx context.Context, tx repo.Transaction) error {
		return tx.RoomMemberRepo().UpdateMember(txCtx, member)
	})
	if err != nil {
		return nil, err
	}

	return &member, nil
}

func (s *RoomMemberService) SetUserRole(ctx context.Context, userId uuid.UUID, roomId uuid.UUID, role domain.Role) error {
	popped, err := s.isRoomPopped(ctx, roomId)
	if err != nil {
		return err
	}
	if popped {
		return errs.ErrRoomPopped
	}

	if s.uow == nil {
		return s.repo.SetUserRole(ctx, userId, roomId, role)
	}

	return s.uow.Do(ctx, func(txCtx context.Context, tx repo.Transaction) error {
		return tx.RoomMemberRepo().SetUserRole(txCtx, userId, roomId, role)
	})
}
func (s *RoomMemberService) InviteUser(ctx context.Context, member structs.RoomUser) error {
	popped, err := s.isRoomPopped(ctx, member.RoomId)
	if err != nil {
		return err
	}
	if popped {
		return errs.ErrRoomPopped
	}

	role := domain.MEMBER
	if member.Role != nil {
		role = *member.Role
	}

	if s.uow == nil {
		return s.repo.AddMember(ctx, domain.RoomMember{
			RoomId:   member.RoomId,
			UserId:   member.UserId,
			Role:     role,
			JoinedAt: time.Time{},
			LeftAt:   nil,
		})
	}

	return s.uow.Do(ctx, func(txCtx context.Context, tx repo.Transaction) error {
		return tx.RoomMemberRepo().AddMember(txCtx, domain.RoomMember{
			RoomId:   member.RoomId,
			UserId:   member.UserId,
			Role:     role,
			JoinedAt: time.Time{},
			LeftAt:   nil,
		})
	})
}

func (s *RoomMemberService) KickUser(ctx context.Context, member structs.RoomUser) error {
	popped, err := s.isRoomPopped(ctx, member.RoomId)
	if err != nil {
		return err
	}
	if popped {
		return errs.ErrRoomPopped
	}

	if s.uow == nil {
		return s.repo.RemoveMember(ctx, member.UserId, member.RoomId)
	}

	return s.uow.Do(ctx, func(txCtx context.Context, tx repo.Transaction) error {
		return tx.RoomMemberRepo().RemoveMember(txCtx, member.UserId, member.RoomId)
	})
}

func (s *RoomMemberService) GetUserByRoomId(ctx context.Context, roomId uuid.UUID) ([]domain.RoomMember, error) {
	return s.repo.GetMembersByRoomId(ctx, roomId)
}

func (s *RoomMemberService) GetUserByRoomIdAndUserId(ctx context.Context, userId uuid.UUID, roomId uuid.UUID) (*domain.RoomMember, error) {
	member, err := s.repo.GetMemberByRoomIdAndMemberId(ctx, roomId, userId)
	if err != nil {
		return nil, err
	}

	return &member, nil
}

func (s *RoomMemberService) GetUserRole(ctx context.Context, userId uuid.UUID, roomId uuid.UUID) (*domain.Role, error) {
	usr, err := s.GetUserByRoomIdAndUserId(ctx, userId, roomId)
	if err != nil || usr == nil {
		return nil, err
	}

	return &usr.Role, nil
}

func (s *RoomMemberService) HasManagerPermission(ctx context.Context, userId uuid.UUID, roomId uuid.UUID) (bool, error) {
	r, err := s.GetUserRole(ctx, userId, roomId)
	if err != nil || r == nil {
		return false, err
	}

	return *r == domain.ADMIN, nil
}

func NewRoomMemberService(roomMemberRepo repo.RoomMemberRepoInterface, roomRepo repo.RoomRepoInterface, uow ...repo.UnitOfWork) *RoomMemberService {
	var unitOfWork repo.UnitOfWork
	if len(uow) > 0 {
		unitOfWork = uow[0]
	}
	return &RoomMemberService{
		repo:     roomMemberRepo,
		roomRepo: roomRepo,
		uow:      unitOfWork,
	}
}
