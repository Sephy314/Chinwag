package repo

import (
	"context"
	"fmt"

	"github.com/jmoiron/sqlx"
)

type Transaction interface {
	UserRepo() UserRepository
	JwtRepo() JwksRepository
}

type UnitOfWork interface {
	Do(ctx context.Context, fn func(ctx context.Context, tx Transaction) error) error
}

type SQLUnitOfWork struct {
	db *sqlx.DB
}

type sqlTransaction struct {
	userRepo UserRepository
	jwtRepo  JwksRepository
}

func (t *sqlTransaction) UserRepo() UserRepository {
	return t.userRepo
}

func (t *sqlTransaction) JwtRepo() JwksRepository {
	return t.jwtRepo
}

func NewSQLUnitOfWork(db *sqlx.DB) *SQLUnitOfWork {
	return &SQLUnitOfWork{db: db}
}

func (u *SQLUnitOfWork) Do(ctx context.Context, fn func(ctx context.Context, tx Transaction) error) error {
	tx, err := u.db.BeginTxx(ctx, nil)
	if err != nil {
		return err
	}

	txObj := &sqlTransaction{
		userRepo: NewUserRepository(tx),
		jwtRepo:  NewJwtRepository(tx),
	}

	if err := fn(ctx, txObj); err != nil {
		if rollbackErr := tx.Rollback(); rollbackErr != nil {
			return fmt.Errorf("transaction failed: %w; rollback failed: %v", err, rollbackErr)
		}
		return err
	}

	return tx.Commit()
}
