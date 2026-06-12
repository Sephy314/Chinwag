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
