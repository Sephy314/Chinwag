package repo

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"strings"
	"time"

	"github.com/Sephy314/chinwag/backend/services/room/domain"
	"github.com/Sephy314/chinwag/backend/services/room/shared/errs"
	"github.com/Sephy314/chinwag/backend/services/room/structs"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

type RoomRepoInterface interface {
	CreateRoom(context.Context, domain.Room) error
	GetRoomById(context.Context, uuid.UUID) (domain.Room, error)
	GetRoomsByOwnerId(context.Context, uuid.UUID) ([]domain.Room, error)
	UpdateRoom(context.Context, domain.Room) error
	DeleteRoomById(context.Context, uuid.UUID) error
	PopRoom(context.Context, uuid.UUID) error
	ListRooms(context.Context, string, int, string) ([]domain.Room, *structs.CursorMeta, error)
	CountRooms(context.Context) (int, error)
	AdminUpdateRoom(context.Context, domain.Room) error
	AdminDeleteRoomById(context.Context, uuid.UUID) error
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
		`INSERT INTO rooms (id, name, description, max_members, owner_id, pop_at) VALUES ($1, $2, $3, $4, $5, $6)`,
		req.Id,
		req.Name,
		req.Description,
		req.MaxMembers,
		req.OwnerId,
		req.PopAt,
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
    			  	 r.pop_at, r.popped_at,
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
       				  r.pop_at, r.popped_at,
       				  r.created_at, r.updated_at, r.deleted_at
				FROM rooms r
				WHERE r.owner_id = $1
					AND deleted_at IS NULL
				ORDER BY r.name`,
		req,
	)
	return rooms, err
}

func (r *RoomRepo) UpdateRoom(ctx context.Context, room domain.Room) error {
	res, err := r.db.ExecContext(
		ctx,
		`UPDATE rooms SET name = $1, description = $2, max_members = $3, updated_at = NOW()
		 WHERE id = $4 AND deleted_at IS NULL AND popped_at IS NULL`,
		room.Name,
		room.Description,
		room.MaxMembers,
		room.Id,
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

func (r *RoomRepo) DeleteRoomById(ctx context.Context, req uuid.UUID) error {
	res, err := r.db.ExecContext(
		ctx,
		`UPDATE rooms SET deleted_at = now() WHERE id = $1 AND popped_at IS NULL`,
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

func (r *RoomRepo) PopRoom(ctx context.Context, roomId uuid.UUID) error {
	res, err := r.db.ExecContext(
		ctx,
		`UPDATE rooms SET popped_at = NOW(), updated_at = NOW()
		 WHERE id = $1 AND popped_at IS NULL AND deleted_at IS NULL`,
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

func (r *RoomRepo) AdminUpdateRoom(ctx context.Context, room domain.Room) error {
	res, err := r.db.ExecContext(
		ctx,
		`UPDATE rooms SET name = $1, description = $2, max_members = $3, updated_at = NOW()
		 WHERE id = $4 AND deleted_at IS NULL`,
		room.Name, room.Description, room.MaxMembers, room.Id,
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

func (r *RoomRepo) AdminDeleteRoomById(ctx context.Context, roomId uuid.UUID) error {
	res, err := r.db.ExecContext(
		ctx,
		`UPDATE rooms SET deleted_at = now() WHERE id = $1`,
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

func (r *RoomRepo) CountRooms(ctx context.Context) (int, error) {
	var n int
	err := sqlx.GetContext(ctx, r.db, &n, `SELECT COUNT(*) FROM rooms WHERE deleted_at IS NULL`)
	return n, err
}

func (r *RoomRepo) ListRooms(ctx context.Context, cursor string, limit int, search string) ([]domain.Room, *structs.CursorMeta, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}

	where := []string{"deleted_at IS NULL"}
	args := []any{}
	arg := func(v any) string {
		args = append(args, v)
		return "?"
	}
	if search != "" {
		where = append(where, "name ILIKE "+arg("%"+search+"%"))
	}

	query := `SELECT id, name, description, max_members, owner_id, pop_at, popped_at, created_at, updated_at, deleted_at
	          FROM rooms WHERE ` + strings.Join(where, " AND ")
	if cursor == "" {
		query += ` ORDER BY created_at DESC, id DESC LIMIT ?`
	} else {
		c, cerr := decodeRoomCursor(cursor)
		if cerr != nil {
			return nil, nil, cerr
		}
		query += ` AND (created_at, id) < (?, ?) ORDER BY created_at DESC, id DESC LIMIT ?`
		args = append(args, c.CreatedAt, c.Id)
	}
	args = append(args, limit+1)
	query = roomRebind(query)

	var rooms []domain.Room
	if err := sqlx.SelectContext(ctx, r.db, &rooms, query, args...); err != nil {
		return nil, nil, err
	}

	hasMore := len(rooms) > limit
	if hasMore {
		rooms = rooms[:limit]
	}
	var meta *structs.CursorMeta
	if hasMore && len(rooms) > 0 {
		last := rooms[len(rooms)-1]
		meta = &structs.CursorMeta{
			NextCursor: encodeRoomCursor(last.CreatedAt, last.Id),
			HasMore:    true,
		}
	}
	return rooms, meta, nil
}

type roomCursor struct {
	CreatedAt time.Time `json:"created_at"`
	Id        uuid.UUID `json:"id"`
}

func encodeRoomCursor(createdAt time.Time, id uuid.UUID) string {
	c := roomCursor{CreatedAt: createdAt, Id: id}
	b, _ := json.Marshal(c)
	return base64.URLEncoding.EncodeToString(b)
}

func decodeRoomCursor(s string) (roomCursor, error) {
	var c roomCursor
	b, err := base64.URLEncoding.DecodeString(s)
	if err != nil {
		return c, err
	}
	err = json.Unmarshal(b, &c)
	return c, err
}

// roomRebind converts "?" placeholders to the pgx ($N) positional style.
func roomRebind(q string) string {
	var b strings.Builder
	n := 0
	for _, r := range q {
		if r == '?' {
			n++
			b.WriteString("$" + itoa(n))
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}
