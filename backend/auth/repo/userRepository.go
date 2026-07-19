package repo

import (
	"context"

	"github.com/Sephy314/chinwag/auth/domain"
	"github.com/Sephy314/chinwag/shared/errs"
	"github.com/jmoiron/sqlx"
)

type UserRepository interface {
	CreateUser(ctx context.Context, user domain.User) error
	GetUser(ctx context.Context, id string) (*domain.User, error)
	UpdateUser(ctx context.Context, user domain.User) error
	DeleteUser(ctx context.Context, id string) error
	GetUserByEmail(ctx context.Context, email string) (*domain.User, error)
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

	if err != nil {
		return err
	}

	return nil
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
