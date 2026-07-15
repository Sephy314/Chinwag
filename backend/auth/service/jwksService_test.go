package service

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"testing"
	"time"

	"github.com/Sephy314/chinwag/auth/domain"
	"github.com/Sephy314/chinwag/shared/utils"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type MockJwtRepository struct {
	mock.Mock
}

func (m *MockJwtRepository) InActiveKey(ctx context.Context, kid string) error {
	args := m.Called(ctx, kid)
	return args.Error(0)
}

func (m *MockJwtRepository) ExpireActiveKey(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *MockJwtRepository) ClearRetiredKeys(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *MockJwtRepository) GetVersion(ctx context.Context) (*time.Time, error) {
	args := m.Called(ctx)

	if args.Get(0) == nil {
		return nil, args.Error(1)
	}

	v := args.Get(0).(time.Time)
	return &v, args.Error(1)
}

func (m *MockJwtRepository) Load(ctx context.Context) ([]domain.SigningKeyEntity, error) {
	args := m.Called(ctx)
	return args.Get(0).([]domain.SigningKeyEntity), args.Error(1)
}

func (m *MockJwtRepository) Rotate(ctx context.Context, key domain.SigningKeyEntity) error {
	args := m.Called(ctx, key)
	return args.Error(0)
}

func (m *MockJwtRepository) GetActiveKey(ctx context.Context) (*domain.SigningKeyEntity, error) {
	args := m.Called(ctx)

	if args.Get(0) == nil {
		return nil, args.Error(1)
	}

	return args.Get(0).(*domain.SigningKeyEntity), args.Error(1)
}

func newTestSigningKey() domain.SigningKeyEntity {
	priv, err := ecdsa.GenerateKey(
		elliptic.P256(),
		rand.Reader,
	)

	if err != nil {
		panic(err)
	}

	pub, err := utils.EncodePublicKey(&priv.PublicKey)
	if err != nil {
		panic(err)
	}

	private, err := utils.EncodePrivateKey(priv)
	if err != nil {
		panic(err)
	}

	now := time.Now()

	return domain.SigningKeyEntity{
		Kid:        "test-kid",
		PublicKey:  pub,
		PrivateKey: private,
		Status:     domain.Active,
		CreatedAt:  now,
		UpdatedAt:  &now,
	}
}
func TestJwksService_LoadJWKS(t *testing.T) {
	repo := new(MockJwtRepository)

	version := time.Now()

	repo.On(
		"GetVersion",
		mock.Anything,
	).Return(version, nil)

	repo.On(
		"Load",
		mock.Anything,
	).Return(
		[]domain.SigningKeyEntity{},
		nil,
	)

	svc := NewJwksService(repo)

	err := svc.LoadJWKS(context.Background())

	require.NoError(t, err)

	repo.AssertExpectations(t)
}
func TestJwksService_LoadJWKS_FirstLoadRotate(t *testing.T) {
	repo := new(MockJwtRepository)

	repo.On(
		"GetVersion",
		mock.Anything,
	).Return(nil, nil)

	repo.On(
		"Rotate",
		mock.Anything,
		mock.Anything,
	).Return(nil)

	repo.On(
		"GetVersion",
		mock.Anything,
	).Return(time.Now(), nil)

	svc := NewJwksService(repo)

	err := svc.LoadJWKS(context.Background())

	require.NoError(t, err)

	repo.AssertExpectations(t)
}

func TestJwksService_GetJwkSet(t *testing.T) {
	repo := new(MockJwtRepository)

	version := time.Now()

	repo.On(
		"GetVersion",
		mock.Anything,
	).Return(version, nil)

	repo.On(
		"Load",
		mock.Anything,
	).Return(
		[]domain.SigningKeyEntity{},
		nil,
	)

	svc := NewJwksService(repo)

	set, err := svc.GetJwkSet(context.Background())

	require.NoError(t, err)
	require.NotNil(t, set)

	repo.AssertExpectations(t)
}

func TestJwksService_GetActiveKey(t *testing.T) {
	repo := new(MockJwtRepository)

	entity := newTestSigningKey()

	version := time.Now()

	repo.On(
		"GetActiveKey",
		mock.Anything,
	).Return(
		&entity,
		nil,
	)

	repo.On(
		"GetVersion",
		mock.Anything,
	).Return(version, nil)

	repo.On(
		"Load",
		mock.Anything,
	).Return(
		[]domain.SigningKeyEntity{},
		nil,
	)

	svc := NewJwksService(repo)

	key, err := svc.GetActiveKey(context.Background())

	require.NoError(t, err)

	require.Equal(
		t,
		entity.Kid,
		key.Kid,
	)

	repo.AssertExpectations(t)
}

func TestJwksService_Rotate(t *testing.T) {
	repo := new(MockJwtRepository)

	version := time.Now()

	repo.On(
		"Rotate",
		mock.Anything,
		mock.MatchedBy(func(key domain.SigningKeyEntity) bool {

			return key.Kid != "" &&
				key.PublicKey != "" &&
				key.PrivateKey != "" &&
				key.Status == domain.Active

		}),
	).Return(nil)

	repo.On(
		"GetVersion",
		mock.Anything,
	).Return(version, nil)

	repo.On(
		"Load",
		mock.Anything,
	).Return(
		[]domain.SigningKeyEntity{},
		nil,
	)

	svc := NewJwksService(repo)

	err := svc.Rotate(context.Background())

	require.NoError(t, err)

	repo.AssertExpectations(t)
}

func TestJwksService_LoadJWKS_NoReload(t *testing.T) {
	repo := new(MockJwtRepository)

	version := time.Now()

	repo.On(
		"GetVersion",
		mock.Anything,
	).Return(version, nil)

	repo.On(
		"Load",
		mock.Anything,
	).Return(
		[]domain.SigningKeyEntity{},
		nil,
	)

	svc := NewJwksService(repo)

	err := svc.LoadJWKS(context.Background())

	require.NoError(t, err)

	repo.AssertExpectations(t)
}

func TestJwksService_LoadJWKS_ActiveKeyExpired(t *testing.T) {
	repo := new(MockJwtRepository)

	now := time.Now()
	expired := now.Add(-time.Hour)

	expiredKeyEntity := newTestSigningKey()
	expiredKeyEntity.Kid = "expired-kid"
	expiredKeyEntity.Status = domain.Active
	expiredKeyEntity.ExpiredAt = &expired

	newKeyEntity := newTestSigningKey()
	newKeyEntity.Kid = "new-kid"
	newKeyEntity.Status = domain.Active

	repo.On(
		"GetVersion",
		mock.Anything,
	).Return(now, nil)

	repo.On(
		"Load",
		mock.Anything,
	).Return(
		[]domain.SigningKeyEntity{expiredKeyEntity},
		nil,
	).Once()

	repo.On(
		"ExpireActiveKey",
		mock.Anything,
	).Return(nil)

	repo.On(
		"Rotate",
		mock.Anything,
		mock.Anything,
	).Return(nil)

	repo.On(
		"Load",
		mock.Anything,
	).Return(
		[]domain.SigningKeyEntity{newKeyEntity},
		nil,
	).Once()

	svc := NewJwksService(repo)

	require.NotNil(t, svc)
	repo.AssertCalled(t, "ExpireActiveKey", mock.Anything)
	repo.AssertCalled(t, "Rotate", mock.Anything, mock.Anything)
	repo.AssertExpectations(t)
}

func TestJwksService_Rotate_ActiveKeyExpired(t *testing.T) {
	repo := new(MockJwtRepository)

	now := time.Now()
	expired := now.Add(-time.Hour)

	expiredKeyEntity := newTestSigningKey()
	expiredKeyEntity.Kid = "expired-kid"
	expiredKeyEntity.Status = domain.Active
	expiredKeyEntity.ExpiredAt = &expired

	newKeyEntity := newTestSigningKey()
	newKeyEntity.Kid = "new-kid"
	newKeyEntity.Status = domain.Active

	repo.On(
		"GetVersion",
		mock.Anything,
	).Return(now, nil)

	repo.On(
		"Load",
		mock.Anything,
	).Return(
		[]domain.SigningKeyEntity{expiredKeyEntity},
		nil,
	).Once()

	repo.On(
		"ExpireActiveKey",
		mock.Anything,
	).Return(nil)

	repo.On(
		"Rotate",
		mock.Anything,
		mock.MatchedBy(func(key domain.SigningKeyEntity) bool {
			return key.Kid != "" &&
				key.PublicKey != "" &&
				key.PrivateKey != "" &&
				key.Status == domain.Active &&
				key.ExpiredAt != nil
		}),
	).Return(nil)

	repo.On(
		"Load",
		mock.Anything,
	).Return(
		[]domain.SigningKeyEntity{newKeyEntity},
		nil,
	).Once()

	svc := NewJwksService(repo)

	require.NotNil(t, svc)
	repo.AssertCalled(t, "ExpireActiveKey", mock.Anything)
	repo.AssertCalled(t, "Rotate", mock.Anything, mock.Anything)
	repo.AssertExpectations(t)
}
