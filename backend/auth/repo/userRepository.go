package repo

import (
	"context"

	"github.com/Sephy314/chinwag/auth/domain"
	"github.com/Sephy314/chinwag/conn"
	"github.com/jmoiron/sqlx"
)

type UserRepository interface {
	CreateUser(ctx context.Context, user domain.User) error
}

func NewUserRepository(conn *conn.Connection) UserRepository {
	repo := UserRepo{
		db: conn.DB,
	}

	return &repo
}

type UserRepo struct {
	db *sqlx.DB
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
