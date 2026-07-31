package repo

import (
	"context"
	"fmt"

	"github.com/jmoiron/sqlx"
)

type Transaction interface {
	ProjectionRepo() ProjectionRepoInterface
}

type UnitOfWork interface {
	Do(ctx context.Context, fn func(ctx context.Context, tx Transaction) error) error
}

type SQLUnitOfWork struct {
	db *sqlx.DB
}

type sqlTransaction struct {
	projectionRepo ProjectionRepoInterface
}

func (t *sqlTransaction) ProjectionRepo() ProjectionRepoInterface {
	return t.projectionRepo
}

func NewSQLUnitOfWork(db *sqlx.DB) *SQLUnitOfWork {
	return &SQLUnitOfWork{db: db}
}

func (u *SQLUnitOfWork) Do(ctx context.Context, fn func(ctx context.Context, tx Transaction) error) error {
	tx, err := u.db.BeginTxx(ctx, nil)
	if err != nil {
		return err
	}

	projectionRepo := NewProjectionRepo(tx)
	txObj := &sqlTransaction{projectionRepo: projectionRepo}

	if err := fn(ctx, txObj); err != nil {
		if rollbackErr := tx.Rollback(); rollbackErr != nil {
			return fmt.Errorf("transaction failed: %w; rollback failed: %v", err, rollbackErr)
		}
		return err
	}

	return tx.Commit()
}
