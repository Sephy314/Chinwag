package mocked

import (
	"context"
	"time"

	"github.com/Sephy314/chinwag/auth/domain"
	"github.com/stretchr/testify/mock"
)

type JwkRepo struct {
	mock.Mock
}

func (m *JwkRepo) Count(ctx context.Context) (*int64, error) {
	args := m.Called(ctx)
	return args.Get(0).(*int64), args.Error(1)
}

func (m *JwkRepo) Load(ctx context.Context) ([]domain.SigningKeyEntity, error) {
	args := m.Called(ctx)
	return args.Get(0).([]domain.SigningKeyEntity), args.Error(1)
}

func (m *JwkRepo) Rotate(ctx context.Context, key domain.SigningKeyEntity) error {
	args := m.Called(ctx, key)
	return args.Error(0)
}

func (m *JwkRepo) InActiveKey(ctx context.Context, kid string) error {
	args := m.Called(ctx, kid)
	return args.Error(0)
}

func (m *JwkRepo) ClearRetiredKeys(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *JwkRepo) GetActiveKey(ctx context.Context) (*domain.SigningKeyEntity, error) {
	args := m.Called(ctx)
	return args.Get(0).(*domain.SigningKeyEntity), args.Error(1)
}

func (m *JwkRepo) GetVersion(ctx context.Context) (*time.Time, error) {
	args := m.Called(ctx)
	return args.Get(0).(*time.Time), args.Error(1)
}
