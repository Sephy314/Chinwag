package service

import (
	"context"
	"net/http"
	"time"

	"github.com/Sephy314/chinwag/room/domain"
	"github.com/Sephy314/chinwag/room/repo"
	"github.com/Sephy314/chinwag/room/structs"
	"github.com/Sephy314/chinwag/shared/errs"
	"github.com/Sephy314/chinwag/shared/patch"
	"github.com/google/uuid"
)

type RoomServiceInterface interface {
	CreateRoom(ctx context.Context, request structs.CreateRoomRequest) (*domain.Room, error)
	GetRoomById(ctx context.Context, roomId uuid.UUID) (*domain.Room, error)
	GetRoomsByOwnerId(ctx context.Context, ownerId uuid.UUID) ([]domain.Room, error)
	UpdateRoom(ctx context.Context, roomId uuid.UUID, req structs.UpdateRoomRequest) (*domain.Room, error)
	DeleteRoom(ctx context.Context, roomId uuid.UUID) error
}

type RoomService struct {
	repo repo.RoomRepoInterface
	uow  repo.UnitOfWork
}

func (r *RoomService) CreateRoom(ctx context.Context, request structs.CreateRoomRequest) (*domain.Room, error) {
	id := uuid.Must(uuid.NewV7())
	now := time.Now()

	ownerId, ok := ctx.Value("ownerId").(uuid.UUID)

	if !ok {
		return nil, &errs.AppError{
			Status:  http.StatusUnauthorized,
			Message: "Unauthorized",
		}
	}

	room := domain.Room{
		Id:          id,
		Name:        request.Name,
		Description: request.Description,
		MaxMembers:  request.MaxMembers,
		OwnerId:     ownerId,
		CreatedAt:   now,
		UpdatedAt:   now,
		DeletedAt:   nil,
	}

	if r.uow == nil {
		err := r.repo.CreateRoom(ctx, room)
		if err != nil {
			return &room, err
		}
		return &room, nil
	}

	err := r.uow.Do(ctx, func(txCtx context.Context, tx repo.Transaction) error {
		if err := tx.RoomRepo().CreateRoom(txCtx, room); err != nil {
			return err
		}

		return tx.RoomMemberRepo().AddMember(txCtx, domain.RoomMember{
			RoomId: room.Id,
			UserId: room.OwnerId,
			Role:   domain.ADMIN,
		})
	})
	if err != nil {
		return &room, err
	}

	return &room, nil
}

func (r *RoomService) GetRoomById(ctx context.Context, roomId uuid.UUID) (*domain.Room, error) {
	room, err := r.repo.GetRoomById(ctx, roomId)
	if err != nil {
		return nil, err
	}

	return &room, nil
}

func (r *RoomService) GetRoomsByOwnerId(ctx context.Context, ownerId uuid.UUID) ([]domain.Room, error) {
	rooms, err := r.repo.GetRoomsByOwnerId(ctx, ownerId)
	if err != nil {
		return nil, err
	}
	return rooms, nil
}

func (r *RoomService) UpdateRoom(ctx context.Context, roomId uuid.UUID, req structs.UpdateRoomRequest) (*domain.Room, error) {
	room, err := r.repo.GetRoomById(ctx, roomId)
	if err != nil {
		return nil, err
	}

	_, err = patch.Patch(&room, req,
		patch.WithIgnore("Id", "OwnerId", "CreatedAt", "UpdatedAt", "DeletedAt"),
	)
	if err != nil {
		return nil, err
	}

	if r.uow == nil {
		err = r.repo.UpdateRoom(ctx, room)
		if err != nil {
			return nil, err
		}
		return &room, nil
	}

	err = r.uow.Do(ctx, func(txCtx context.Context, tx repo.Transaction) error {
		return tx.RoomRepo().UpdateRoom(txCtx, room)
	})
	if err != nil {
		return nil, err
	}

	return &room, nil
}

func (r *RoomService) DeleteRoom(ctx context.Context, roomId uuid.UUID) error {
	if r.uow == nil {
		return r.repo.DeleteRoomById(ctx, roomId)
	}

	return r.uow.Do(ctx, func(txCtx context.Context, tx repo.Transaction) error {
		return tx.RoomRepo().DeleteRoomById(txCtx, roomId)
	})
}

func NewRoomService(roomRepo repo.RoomRepoInterface, uow ...repo.UnitOfWork) *RoomService {
	var unitOfWork repo.UnitOfWork
	if len(uow) > 0 {
		unitOfWork = uow[0]
	}
	return &RoomService{
		repo: roomRepo,
		uow:  unitOfWork,
	}
}
