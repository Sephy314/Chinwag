package repo

import (
	"context"

	"github.com/Sephy314/chinwag/room/domain"
	"github.com/Sephy314/chinwag/shared/errs"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

type RoomMemberRepoInterface interface {
	GetMembersByRoomId(context.Context, uuid.UUID) ([]domain.RoomMember, error)
	GetRoomsByUserId(context.Context, uuid.UUID) ([]domain.Room, error)
	GetMemberByRoomIdAndMemberId(context.Context, uuid.UUID, uuid.UUID) (domain.RoomMember, error)
	AddMember(context.Context, domain.RoomMember) error
	RemoveMember(context.Context, uuid.UUID, uuid.UUID) error
	SetUserRole(context.Context, uuid.UUID, uuid.UUID, domain.Role) error
}

type RoomMemberRepo struct {
	db sqlx.ExtContext
}

func NewRoomMemberRepo(db sqlx.ExtContext) *RoomMemberRepo {
	return &RoomMemberRepo{db: db}
}

func (r *RoomMemberRepo) GetMembersByRoomId(ctx context.Context, userId uuid.UUID) ([]domain.RoomMember, error) {
	var members []domain.RoomMember
	err := sqlx.SelectContext(
		ctx,
		r.db,
		&members,
		`SELECT  
    			r.user_id, r.room_id, r.role, r.joined_at
    			FROM room_member r
    			WHERE r.room_id = $1
    				AND r.left_at IS NULL`,
		userId,
	)

	return members, err
}

func (r *RoomMemberRepo) GetMemberByRoomIdAndMemberId(ctx context.Context, userId uuid.UUID, roomId uuid.UUID) (domain.RoomMember, error) {
	var member domain.RoomMember
	err := sqlx.GetContext(
		ctx,
		r.db,
		&member,
		`SELECT 
    				 r.user_id, r.room_id, r.role, r.joined_at
				FROM room_member r
				WHERE r.room_id = $1
					  AND r.user_id = $2  
				      AND r.left_at IS NULL
				LIMIT 1`,
		roomId,
		userId,
	)

	return member, err
}

func (r *RoomMemberRepo) AddMember(ctx context.Context, req domain.RoomMember) error {
	res, err := r.db.ExecContext(
		ctx,
		`INSERT INTO room_member (user_id, room_id, role) VALUES ($1, $2, $3)`,
		req.UserId,
		req.RoomId,
		req.Role,
	)
	if err != nil {
		return err
	}

	rows, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return errs.ErrConflict
	}

	return nil
}

func (r *RoomMemberRepo) RemoveMember(ctx context.Context, userId uuid.UUID, roomId uuid.UUID) error {
	res, err := r.db.ExecContext(
		ctx,
		`UPDATE room_member SET left_at = now() WHERE user_id = $1 AND room_id = $2`,
		userId,
		roomId,
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

func (r *RoomMemberRepo) SetUserRole(ctx context.Context, userId uuid.UUID, roomId uuid.UUID, role domain.Role) error {
	res, err := r.db.ExecContext(
		ctx,
		`UPDATE room_member
			   SET role = $1
			   WHERE user_id = $2 AND room_id = $3
			   `,
		role,
		userId,
		roomId,
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

func (r *RoomMemberRepo) GetRoomsByUserId(ctx context.Context, userId uuid.UUID) ([]domain.Room, error) {
	var rooms []domain.Room

	err := sqlx.SelectContext(
		ctx,
		r.db,
		&rooms,
		`SELECT rm.id, rm.name, rm.description, rm.max_members, rm.owner_id, rm.created_at, rm.updated_at, rm.deleted_at
				FROM room_member r
				JOIN rooms rm ON rm.id = r.room_id
				WHERE r.user_id = $1
					AND r.left_at IS NULL
					AND rm.deleted_at IS NULL
					ORDER BY r.joined_at DESC`,
		userId,
	)

	return rooms, err
}
