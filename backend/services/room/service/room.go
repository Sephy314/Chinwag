package service

import (
	"context"
	"net/http"
	"time"

	"github.com/Sephy314/chinwag/backend/services/room/domain"
	"github.com/Sephy314/chinwag/backend/services/room/repo"
	"github.com/Sephy314/chinwag/backend/services/room/shared/errs"
	"github.com/Sephy314/chinwag/backend/services/room/shared/patch"
	"github.com/Sephy314/chinwag/backend/services/room/structs"
	"github.com/google/uuid"
)

type RoomServiceInterface interface {
	CreateRoom(ctx context.Context, request structs.CreateRoomRequest) (*domain.Room, error)
	GetRoomById(ctx context.Context, roomId uuid.UUID) (*domain.Room, error)
	GetRoomsByOwnerId(ctx context.Context, ownerId uuid.UUID) ([]domain.Room, error)
	UpdateRoom(ctx context.Context, roomId uuid.UUID, req structs.UpdateRoomRequest) (*domain.Room, error)
	DeleteRoom(ctx context.Context, roomId uuid.UUID) error
	PopRoom(ctx context.Context, roomId uuid.UUID) error
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

	// pop_at is optional. When omitted, PopAt stays nil and the room has no
	// auto-pop schedule (the scheduler skips NULL pop_at).
	var popAt *time.Time
	if request.PopAt != nil {
		if request.PopAt.Before(now) {
			return nil, &errs.AppError{
				Status:  http.StatusBadRequest,
				Message: "pop_at must be in the future",
			}
		}
		popAt = request.PopAt
	}

	room := domain.Room{
		Id:          id,
		Name:        request.Name,
		Description: request.Description,
		MaxMembers:  request.MaxMembers,
		OwnerId:     ownerId,
		PopAt:       popAt,
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

	if room.PoppedAt != nil {
		return nil, errs.ErrRoomPopped
	}

	_, err = patch.Patch(&room, req,
		patch.WithIgnore("Id", "OwnerId", "CreatedAt", "UpdatedAt", "DeletedAt", "PopAt", "PoppedAt"),
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
	room, err := r.repo.GetRoomById(ctx, roomId)
	if err != nil {
		return err
	}

	if room.PoppedAt != nil {
		return errs.ErrRoomPopped
	}

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

// --- Admin operations ---

func (r *RoomService) AdminListRooms(ctx context.Context, req structs.ListRoomsRequest) ([]domain.Room, *structs.CursorMeta, error) {
	return r.repo.ListRooms(ctx, req.Cursor, req.Limit, req.Search)
}

func (r *RoomService) AdminGetRoom(ctx context.Context, roomId uuid.UUID) (*domain.Room, error) {
	return r.GetRoomById(ctx, roomId)
}

// AdminCreateRoom creates a room as an admin. The owner is the acting admin
// unless an explicit OwnerId is provided, mirroring the normal creation flow
// (owner is added as a room ADMIN member).
func (r *RoomService) AdminCreateRoom(ctx context.Context, request structs.AdminCreateRoomRequest, actorID uuid.UUID) (*domain.Room, error) {
	id := uuid.Must(uuid.NewV7())
	now := time.Now()

	ownerId := actorID
	if request.OwnerId != nil {
		ownerId = *request.OwnerId
	}

	var popAt *time.Time
	if request.PopAt != nil {
		if request.PopAt.Before(now) {
			return nil, &errs.AppError{
				Status:  http.StatusBadRequest,
				Message: "pop_at must be in the future",
			}
		}
		popAt = request.PopAt
	}

	room := domain.Room{
		Id:          id,
		Name:        request.Name,
		Description: request.Description,
		MaxMembers:  request.MaxMembers,
		OwnerId:     ownerId,
		PopAt:       popAt,
		CreatedAt:   now,
		UpdatedAt:   now,
		DeletedAt:   nil,
	}

	if r.uow == nil {
		if err := r.repo.CreateRoom(ctx, room); err != nil {
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
			UserId: ownerId,
			Role:   domain.ADMIN,
		})
	})
	if err != nil {
		return &room, err
	}
	return &room, nil
}

// AdminUpdateRoom updates room metadata even for popped rooms (normal users
// cannot). It preserves the repository's persistence rules.
func (r *RoomService) AdminUpdateRoom(ctx context.Context, roomId uuid.UUID, req structs.AdminUpdateRoomRequest) (*domain.Room, error) {
	room, err := r.repo.GetRoomById(ctx, roomId)
	if err != nil {
		return nil, err
	}

	_, err = patch.Patch(&room, req,
		patch.WithIgnore("Id", "OwnerId", "CreatedAt", "UpdatedAt", "DeletedAt", "PopAt", "PoppedAt"),
	)
	if err != nil {
		return nil, err
	}

	if err := r.repo.AdminUpdateRoom(ctx, room); err != nil {
		return nil, err
	}
	return &room, nil
}

// AdminDeleteRoom soft-deletes a room even if it has been popped.
func (r *RoomService) AdminDeleteRoom(ctx context.Context, roomId uuid.UUID) error {
	return r.repo.AdminDeleteRoomById(ctx, roomId)
}

func (r *RoomService) AdminCountRooms(ctx context.Context) (int64, error) {
	n, err := r.repo.CountRooms(ctx)
	return int64(n), err
}

func (r *RoomService) PopRoom(ctx context.Context, roomId uuid.UUID) error {
	room, err := r.repo.GetRoomById(ctx, roomId)
	if err != nil {
		return err
	}

	if room.PoppedAt != nil {
		return errs.ErrRoomPopped
	}

	return r.repo.PopRoom(ctx, roomId)
}
