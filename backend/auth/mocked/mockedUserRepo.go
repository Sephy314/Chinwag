package mocked

import (
	"context"

	"github.com/Sephy314/chinwag/auth/domain"
	"github.com/stretchr/testify/mock"
)

type UserRepo struct {
	mock.Mock
}

func (m *UserRepo) CreateUser(ctx context.Context, user domain.User) error {
	args := m.Called(ctx, user)
	return args.Error(0)
}

func (m *UserRepo) GetUser(ctx context.Context, id string) (*domain.User, error) {
	args := m.Called(ctx, id)

	user, ok := args.Get(0).(*domain.User)
	if !ok {
		return nil, args.Error(1)
	}

	return user, args.Error(1)
}

func (m *UserRepo) DeleteUser(ctx context.Context, id string) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}
