package repo

import (
	"context"
	"fmt"

	"github.com/jmoiron/sqlx"
)

type Transaction interface {
	RoomRepo() RoomRepoInterface
	RoomMemberRepo() RoomMemberRepoInterface
}

type UnitOfWork interface {
	Do(ctx context.Context, fn func(ctx context.Context, tx Transaction) error) error
}

type SQLUnitOfWork struct {
	db *sqlx.DB
}

type sqlTransaction struct {
	roomRepo       RoomRepoInterface
	roomMemberRepo RoomMemberRepoInterface
}

func (t *sqlTransaction) RoomRepo() RoomRepoInterface {
	return t.roomRepo
}

func (t *sqlTransaction) RoomMemberRepo() RoomMemberRepoInterface {
	return t.roomMemberRepo
}

func NewSQLUnitOfWork(db *sqlx.DB) *SQLUnitOfWork {
	return &SQLUnitOfWork{db: db}
}

func (u *SQLUnitOfWork) Do(ctx context.Context, fn func(ctx context.Context, tx Transaction) error) error {
	tx, err := u.db.BeginTxx(ctx, nil)
	if err != nil {
		return err
	}

	roomRepo := NewRoomRepo(tx)
	roomMemberRepo := NewRoomMemberRepo(tx)
	txObj := &sqlTransaction{
		roomRepo:       roomRepo,
		roomMemberRepo: roomMemberRepo,
	}

	if err := fn(ctx, txObj); err != nil {
		if rollbackErr := tx.Rollback(); rollbackErr != nil {
			return fmt.Errorf("transaction failed: %w; rollback failed: %v", err, rollbackErr)
		}
		return err
	}

	return tx.Commit()
}
