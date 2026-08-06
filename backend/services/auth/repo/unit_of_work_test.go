package repo

import (
	"context"
	"errors"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestUow(t *testing.T) (*SQLUnitOfWork, sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	return NewSQLUnitOfWork(sqlx.NewDb(db, "sqlmock")), mock
}

func TestSQLUnitOfWork_Do_BeginFails_WhenDBDown(t *testing.T) {
	uow, mock := newTestUow(t)
	mock.ExpectBegin().WillReturnError(errors.New("connection refused"))

	called := false
	err := uow.Do(context.Background(), func(ctx context.Context, tx Transaction) error {
		called = true
		return nil
	})

	assert.EqualError(t, err, "connection refused")
	assert.False(t, called, "fn must not run when the transaction cannot begin")
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestSQLUnitOfWork_Do_FnError_RollsBack(t *testing.T) {
	uow, mock := newTestUow(t)
	mock.ExpectBegin()
	mock.ExpectRollback()

	sentinel := errors.New("business rule violated")
	err := uow.Do(context.Background(), func(ctx context.Context, tx Transaction) error {
		return sentinel
	})

	assert.ErrorIs(t, err, sentinel)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestSQLUnitOfWork_Do_FnError_RollbackFails(t *testing.T) {
	uow, mock := newTestUow(t)
	mock.ExpectBegin()
	mock.ExpectRollback().WillReturnError(errors.New("rollback failed"))

	sentinel := errors.New("boom")
	err := uow.Do(context.Background(), func(ctx context.Context, tx Transaction) error {
		return sentinel
	})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "transaction failed: boom")
	assert.Contains(t, err.Error(), "rollback failed")
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestSQLUnitOfWork_Do_Success_Commits(t *testing.T) {
	uow, mock := newTestUow(t)
	mock.ExpectBegin()
	mock.ExpectCommit()

	err := uow.Do(context.Background(), func(ctx context.Context, tx Transaction) error {
		return nil
	})

	assert.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}
