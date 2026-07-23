package service

import (
	"context"
	"errors"
	"testing"

	"github.com/Sephy314/chinwag/auth/domain"
	"github.com/Sephy314/chinwag/auth/mocked"
	"github.com/Sephy314/chinwag/auth/repo"
	"github.com/Sephy314/chinwag/auth/structs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type stubAuthTransaction struct {
	userRepo repo.UserRepository
	jwtRepo  repo.JwksRepository
}

func (t *stubAuthTransaction) UserRepo() repo.UserRepository {
	return t.userRepo
}

func (t *stubAuthTransaction) JwtRepo() repo.JwksRepository {
	return t.jwtRepo
}

type stubAuthUnitOfWork struct {
	tx     repo.Transaction
	called bool
	err    error
}

func (u *stubAuthUnitOfWork) Do(ctx context.Context, fn func(ctx context.Context, tx repo.Transaction) error) error {
	u.called = true
	if u.err != nil {
		return u.err
	}
	return fn(ctx, u.tx)
}

func TestUserService_CreateUser_UsesUnitOfWork(t *testing.T) {
	ctx := context.Background()
	txRepo := new(mocked.UserRepo)

	req := structs.CreateUserReq{
		Name:     "tester",
		Email:    "tester@example.com",
		Password: "password123",
	}

	txRepo.On("CreateUser", ctx, mock.MatchedBy(func(user domain.User) bool {
		return user.Name == "tester" && user.Email == "tester@example.com"
	})).Return(nil)

	service := &UserService{
		Repo: new(mocked.UserRepo),
		uow: &stubAuthUnitOfWork{
			tx: &stubAuthTransaction{
				userRepo: txRepo,
			},
		},
	}

	created, err := service.CreateUser(ctx, req)
	assert.NoError(t, err)
	assert.NotNil(t, created)
	assert.True(t, service.uow.(*stubAuthUnitOfWork).called)

	txRepo.AssertExpectations(t)
}

func TestUserService_DeleteUser_UsesUnitOfWork(t *testing.T) {
	ctx := context.Background()
	txRepo := new(mocked.UserRepo)

	txRepo.On("DeleteUser", ctx, "user-123").Return(nil)

	service := &UserService{
		Repo: new(mocked.UserRepo),
		uow: &stubAuthUnitOfWork{
			tx: &stubAuthTransaction{
				userRepo: txRepo,
			},
		},
	}

	err := service.DeleteUser(ctx, "user-123")
	assert.NoError(t, err)
	assert.True(t, service.uow.(*stubAuthUnitOfWork).called)

	txRepo.AssertExpectations(t)
}

func TestUserService_UnitOfWorkErrorIsReturned(t *testing.T) {
	ctx := context.Background()
	uowErr := errors.New("tx failed")

	service := &UserService{
		Repo: new(mocked.UserRepo),
		uow:  &stubAuthUnitOfWork{err: uowErr},
	}

	_, err := service.CreateUser(ctx, structs.CreateUserReq{
		Name:     "tester",
		Email:    "tester@example.com",
		Password: "password123",
	})

	assert.ErrorIs(t, err, uowErr)
}
