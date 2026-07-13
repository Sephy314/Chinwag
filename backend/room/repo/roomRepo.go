package repo

import (
	"context"

	"github.com/Sephy314/chinwag/room/domain"
	"github.com/Sephy314/chinwag/shared/errs"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

type RoomRepoInterface interface {
	CreateRoom(context.Context, domain.Room) error
	GetRoomById(context.Context, uuid.UUID) (domain.Room, error)
	GetRoomsByOwnerId(context.Context, uuid.UUID) ([]domain.Room, error)
	DeleteRoomById(context.Context, uuid.UUID) error
}

type RoomRepo struct {
	db sqlx.ExtContext
}

func NewRoomRepo(db sqlx.ExtContext) *RoomRepo {
	return &RoomRepo{db: db}
}

func (r *RoomRepo) CreateRoom(ctx context.Context, req domain.Room) error {
	_, err := r.db.ExecContext(
		ctx,
		`INSERT INTO rooms (id, name, description, max_members, owner_id) VALUES ($1, $2, $3, $4, $5)`,
		req.Id,
		req.Name,
		req.Description,
		req.MaxMembers,
		req.OwnerId,
	)

	return err
}

func (r *RoomRepo) GetRoomById(ctx context.Context, req uuid.UUID) (domain.Room, error) {
	var room domain.Room
	err := sqlx.GetContext(
		ctx,
		r.db,
		&room,
		`SELECT 
    				 r.id, r.name, r.description, r.max_members, r.owner_id, 
    			 	 r.created_at, r.updated_at, r.deleted_at
    			FROM rooms r
    			WHERE id = $1 AND
    				deleted_at IS NULL`,
		req,
	)
	return room, err
}

func (r *RoomRepo) GetRoomsByOwnerId(ctx context.Context, req uuid.UUID) ([]domain.Room, error) {
	var rooms []domain.Room
	err := sqlx.SelectContext(
		ctx,
		r.db,
		&rooms,
		`SELECT r.id, r.name, r.description, r.max_members, r.owner_id, 
       				  r.created_at, r.updated_at, r.deleted_at
				FROM rooms r
				WHERE r.owner_id = $1
					AND deleted_at IS NULL
				ORDER BY r.name`,
		req,
	)
	return rooms, err
}

func (r *RoomRepo) DeleteRoomById(ctx context.Context, req uuid.UUID) error {
	res, err := r.db.ExecContext(
		ctx,
		`UPDATE rooms SET deleted_at = now() WHERE id = $1`,
		req,
	)

	if err != nil {
		return err
	}

	rows, err := res.RowsAffected()
	if err != nil {
		return err
	}

	if rows == 0 {
		return errs.ErrNotFound
	}

	return nil
}
