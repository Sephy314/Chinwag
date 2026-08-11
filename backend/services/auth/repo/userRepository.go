package repo

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"strings"
	"time"

	"github.com/Sephy314/chinwag/backend/services/auth/domain"
	"github.com/Sephy314/chinwag/backend/services/auth/shared/errs"
	"github.com/Sephy314/chinwag/backend/services/auth/structs"
	"github.com/jmoiron/sqlx"
)

type UserRepository interface {
	CreateUser(ctx context.Context, user domain.User) error
	CreateOAuthUser(ctx context.Context, user domain.User) error
	GetUser(ctx context.Context, id string) (*domain.User, error)
	GetUserIncludingDeleted(ctx context.Context, id string) (*domain.User, error)
	UpdateUser(ctx context.Context, user domain.User) error
	DeleteUser(ctx context.Context, id string) error
	RestoreUser(ctx context.Context, id string) error
	SetRole(ctx context.Context, id string, role domain.Role) error
	GetUserByEmail(ctx context.Context, email string) (*domain.User, error)
	ListUsers(ctx context.Context, cursor string, limit int, role, deleted, search string) ([]domain.User, *structs.CursorMeta, error)
	CountUsers(ctx context.Context) (int, error)
	CountAdmins(ctx context.Context) (int, error)
}

func NewUserRepository(db sqlx.ExtContext) UserRepository {
	return &UserRepo{db: db}
}

type UserRepo struct {
	db sqlx.ExtContext
}

func (r *UserRepo) GetUser(ctx context.Context, id string) (*domain.User, error) {
	var user domain.User

	err := sqlx.GetContext(ctx, r.db, &user, `SELECT * FROM users WHERE id = $1 AND deleted_at IS NULL`, id)
	if err != nil {
		return nil, err
	}

	return &user, nil
}

func (r *UserRepo) GetUserIncludingDeleted(ctx context.Context, id string) (*domain.User, error) {
	var user domain.User

	err := sqlx.GetContext(ctx, r.db, &user, `SELECT * FROM users WHERE id = $1`, id)
	if err != nil {
		return nil, err
	}

	return &user, nil
}

func (r *UserRepo) RestoreUser(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx, `UPDATE users SET deleted_at = NULL, updated_at = NOW() WHERE id = $1`, id)
	return err
}

func (r *UserRepo) SetRole(ctx context.Context, id string, role domain.Role) error {
	_, err := r.db.ExecContext(ctx, `UPDATE users SET role = $1, updated_at = NOW() WHERE id = $2`, role, id)
	return err
}

func (r *UserRepo) CountUsers(ctx context.Context) (int, error) {
	var n int
	err := sqlx.GetContext(ctx, r.db, &n, `SELECT COUNT(*) FROM users WHERE deleted_at IS NULL`)
	return n, err
}

func (r *UserRepo) CountAdmins(ctx context.Context) (int, error) {
	var n int
	err := sqlx.GetContext(ctx, r.db, &n, `SELECT COUNT(*) FROM users WHERE role = 'ADMIN' AND deleted_at IS NULL`)
	return n, err
}

func (r *UserRepo) ListUsers(ctx context.Context, cursor string, limit int, role, deleted, search string) ([]domain.User, *structs.CursorMeta, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}

	where := []string{"1=1"}
	args := []any{}
	arg := func(v any) string {
		args = append(args, v)
		return "?"
	}

	switch deleted {
	case "only":
		where = append(where, "deleted_at IS NOT NULL")
	case "include":
		// no-op: include both active and deleted
	default:
		where = append(where, "deleted_at IS NULL")
	}

	if role != "" {
		where = append(where, "role = "+arg(role))
	}
	if search != "" {
		where = append(where, "(name ILIKE "+arg("%"+search+"%")+" OR email ILIKE "+arg("%"+search+"%")+")")
	}

	query := `SELECT * FROM users WHERE ` + userRebind(strings.Join(where, " AND "))
	if cursor == "" {
		query += ` ORDER BY created_at DESC, id DESC LIMIT ?`
	} else {
		c, cerr := decodeUserCursor(cursor)
		if cerr != nil {
			return nil, nil, cerr
		}
		query += ` AND (created_at, id) < (?, ?) ORDER BY created_at DESC, id DESC LIMIT ?`
		args = append(args, c.CreatedAt, c.Id)
	}
	args = append(args, limit+1)
	query = userRebind(query)

	var users []domain.User
	if err := sqlx.SelectContext(ctx, r.db, &users, query, args...); err != nil {
		return nil, nil, err
	}

	hasMore := len(users) > limit
	if hasMore {
		users = users[:limit]
	}

	var meta *structs.CursorMeta
	if hasMore && len(users) > 0 {
		last := users[len(users)-1]
		meta = &structs.CursorMeta{
			NextCursor: encodeUserCursor(last.CreatedAt, last.Id),
			HasMore:    true,
		}
	}

	return users, meta, nil
}

func (r *UserRepo) DeleteUser(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx, `UPDATE users SET deleted_at = NOW() WHERE id = $1`, id)
	return err
}

func (r *UserRepo) CreateUser(ctx context.Context, user domain.User) error {
	_, err := r.db.ExecContext(
		ctx,
		"INSERT INTO users (id, name, email, password) VALUES ($1, $2, $3, $4)",
		user.Id,
		user.Name,
		user.Email,
		user.Password,
	)
	return err
}

func (r *UserRepo) CreateOAuthUser(ctx context.Context, user domain.User) error {
	_, err := r.db.ExecContext(
		ctx,
		`INSERT INTO users (id, name, email, password, provider, provider_id)
		 VALUES ($1, $2, $3, $4, $5, $6)
		 ON CONFLICT (email) DO UPDATE SET
		   name = EXCLUDED.name,
		   provider = EXCLUDED.provider,
		   provider_id = EXCLUDED.provider_id,
		   deleted_at = NULL,
		   updated_at = NOW()`,
		user.Id,
		user.Name,
		user.Email,
		user.Password,
		user.Provider,
		user.ProviderID,
	)
	return err
}

func (r *UserRepo) GetUserByEmail(ctx context.Context, email string) (*domain.User, error) {
	var user domain.User

	err := sqlx.GetContext(
		ctx,
		r.db,
		&user,
		"SELECT * FROM users WHERE email = $1 AND deleted_at IS NULL",
		email,
	)
	if err != nil {
		return nil, err
	}

	return &user, nil
}

func (r *UserRepo) UpdateUser(ctx context.Context, user domain.User) error {
	res, err := r.db.ExecContext(ctx,
		`UPDATE users SET name = $1, email = $2, password = $3, updated_at = NOW() WHERE id = $4 AND deleted_at IS NULL`,
		user.Name, user.Email, user.Password, user.Id)
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

type userCursor struct {
	CreatedAt time.Time `json:"created_at"`
	Id        string    `json:"id"`
}

func encodeUserCursor(createdAt time.Time, id string) string {
	c := userCursor{CreatedAt: createdAt, Id: id}
	b, _ := json.Marshal(c)
	return base64.URLEncoding.EncodeToString(b)
}

func decodeUserCursor(s string) (userCursor, error) {
	var c userCursor
	b, err := base64.URLEncoding.DecodeString(s)
	if err != nil {
		return c, err
	}
	err = json.Unmarshal(b, &c)
	return c, err
}

// userRebind converts "?" placeholders to the pgx ($N) positional style used
// by this codebase.
func userRebind(q string) string {
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
