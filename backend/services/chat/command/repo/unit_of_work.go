package repo

import (
	"context"
	"fmt"

	"github.com/jmoiron/sqlx"
)

type Transaction interface {
	ChatRepo() ChatRepoInterface
	OutboxRepo() OutboxRepoInterface
}

type UnitOfWork interface {
	Do(ctx context.Context, fn func(ctx context.Context, tx Transaction) error) error
}

type SQLUnitOfWork struct {
	db *sqlx.DB
}

type sqlTransaction struct {
	chatRepo   ChatRepoInterface
	outboxRepo OutboxRepoInterface
}

func (t *sqlTransaction) ChatRepo() ChatRepoInterface {
	return t.chatRepo
}

func (t *sqlTransaction) OutboxRepo() OutboxRepoInterface {
	return t.outboxRepo
}

func NewSQLUnitOfWork(db *sqlx.DB) *SQLUnitOfWork {
	return &SQLUnitOfWork{db: db}
}

func (u *SQLUnitOfWork) Do(ctx context.Context, fn func(ctx context.Context, tx Transaction) error) error {
	tx, err := u.db.BeginTxx(ctx, nil)
	if err != nil {
		return err
	}

	chatRepo := NewChatRepo(tx)
	outboxRepo := NewOutboxRepo(tx)
	txObj := &sqlTransaction{
		chatRepo:   chatRepo,
		outboxRepo: outboxRepo,
	}

	if err := fn(ctx, txObj); err != nil {
		if rollbackErr := tx.Rollback(); rollbackErr != nil {
			return fmt.Errorf("transaction failed: %w; rollback failed: %v", err, rollbackErr)
		}
		return err
	}

	return tx.Commit()
}
