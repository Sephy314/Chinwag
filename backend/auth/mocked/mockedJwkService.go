package mocked

import (
	"context"

	"github.com/Sephy314/chinwag/auth/domain"
	"github.com/lestrrat-go/jwx/v3/jwk"
	"github.com/stretchr/testify/mock"
)

// JwkService is a testify mock that implements the JwksServiceImpl methods used in tests
type JwkService struct {
	mock.Mock
}

func (m *JwkService) LoadJWKS(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *JwkService) GetJwkSet(ctx context.Context) (jwk.Set, error) {
	args := m.Called(ctx)
	var set jwk.Set
	if args.Get(0) != nil {
		set = args.Get(0).(jwk.Set)
	}
	return set, args.Error(1)
}

func (m *JwkService) Rotate(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *JwkService) GetActiveKey(ctx context.Context) (*domain.SigningKeyEntity, error) {
	args := m.Called(ctx)
	var key *domain.SigningKeyEntity
	if args.Get(0) != nil {
		key = args.Get(0).(*domain.SigningKeyEntity)
	}
	return key, args.Error(1)
}
