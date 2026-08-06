package service

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"errors"
	"testing"
	"time"

	"github.com/Sephy314/chinwag/backend/services/auth/domain"
	internaljwt "github.com/Sephy314/chinwag/backend/services/auth/internal/jwt"
	"github.com/Sephy314/chinwag/backend/services/auth/shared/errs"
	"github.com/lestrrat-go/jwx/v3/jwk"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func makeKeyEntity(t *testing.T, kid string, status domain.KeyStatus, expiry time.Time) domain.SigningKeyEntity {
	t.Helper()
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)

	pubB64, err := internaljwt.EncodePublicKey(&priv.PublicKey)
	require.NoError(t, err)
	privB64, err := internaljwt.EncodePrivateKey(priv)
	require.NoError(t, err)

	now := time.Now()
	return domain.SigningKeyEntity{
		Kid:        kid,
		PublicKey:  pubB64,
		PrivateKey: privB64,
		Status:     status,
		CreatedAt:  now,
		UpdatedAt:  &now,
		ExpiredAt:  &expiry,
	}
}

func newTestJwksService(repo *MockJwksRepo) *JwksService {
	return &JwksService{
		jwkSet: jwk.NewSet(),
		repo:   repo,
		log:    &MockLogger{},
	}
}

func TestJwksService_LoadJWKS_NoVersion_Rotates(t *testing.T) {
	repo := new(MockJwksRepo)
	svc := newTestJwksService(repo)

	repo.On("GetVersion", mock.Anything).Return(nil, nil).Once() // no version
	repo.On("Rotate", mock.Anything, mock.Anything).Return(nil).Once()
	repo.On("GetVersion", mock.Anything).Return(nil, nil).Once() // still nil

	err := svc.LoadJWKS(context.Background())

	assert.NoError(t, err)
	repo.AssertExpectations(t)
}

func TestJwksService_LoadJWKS_ReloadsOnNewVersion(t *testing.T) {
	repo := new(MockJwksRepo)
	svc := newTestJwksService(repo)

	now := time.Now()
	future := now.Add(24 * time.Hour)
	entities := []domain.SigningKeyEntity{makeKeyEntity(t, "kid1", domain.Active, future)}

	repo.On("GetVersion", mock.Anything).Return(&now, nil).Once()
	repo.On("Load", mock.Anything).Return(entities, nil).Once()

	err := svc.LoadJWKS(context.Background())

	assert.NoError(t, err)
	assert.Equal(t, now, svc.version)
	// note: jwx v3's jwk.Set.Keys() returns private params (always empty), so
	// we cannot assert on it; the reload is verified via repo.Load + version.
	repo.AssertExpectations(t)
}

func TestJwksService_LoadJWKS_GetVersionError(t *testing.T) {
	repo := new(MockJwksRepo)
	svc := newTestJwksService(repo)

	repo.On("GetVersion", mock.Anything).Return(nil, errors.New("db down")).Once()

	err := svc.LoadJWKS(context.Background())

	assert.EqualError(t, err, "db down")
	repo.AssertNotCalled(t, "Load", mock.Anything)
	repo.AssertExpectations(t)
}

func TestJwksService_LoadJWKS_LoadError(t *testing.T) {
	repo := new(MockJwksRepo)
	svc := newTestJwksService(repo)

	now := time.Now()
	repo.On("GetVersion", mock.Anything).Return(&now, nil).Once()
	repo.On("Load", mock.Anything).Return(nil, errors.New("db down")).Once()

	err := svc.LoadJWKS(context.Background())

	assert.EqualError(t, err, "db down")
	repo.AssertExpectations(t)
}

func TestJwksService_LoadJWKS_ActiveKeyExpired_Rotates(t *testing.T) {
	repo := new(MockJwksRepo)
	svc := newTestJwksService(repo)

	now := time.Now()
	past := now.Add(-time.Hour)
	future := now.Add(24 * time.Hour)

	expired := makeKeyEntity(t, "expired", domain.Active, past)
	fresh := makeKeyEntity(t, "fresh", domain.Active, future)

	repo.On("GetVersion", mock.Anything).Return(&now, nil).Once()
	repo.On("Load", mock.Anything).Return([]domain.SigningKeyEntity{expired}, nil).Once()
	repo.On("ExpireActiveKey", mock.Anything).Return(nil).Once()
	repo.On("Rotate", mock.Anything, mock.Anything).Return(nil).Once()
	repo.On("Load", mock.Anything).Return([]domain.SigningKeyEntity{fresh}, nil).Once()

	err := svc.LoadJWKS(context.Background())

	assert.NoError(t, err)
	repo.AssertExpectations(t)
}

func TestJwksService_GetJwkSet_Success(t *testing.T) {
	repo := new(MockJwksRepo)
	svc := newTestJwksService(repo)

	now := time.Now()
	future := now.Add(24 * time.Hour)
	entities := []domain.SigningKeyEntity{makeKeyEntity(t, "kid1", domain.Active, future)}

	repo.On("GetVersion", mock.Anything).Return(&now, nil).Once()
	repo.On("Load", mock.Anything).Return(entities, nil).Once()

	set, err := svc.GetJwkSet(context.Background())

	assert.NoError(t, err)
	assert.NotNil(t, set)
	repo.AssertExpectations(t)
}

func TestJwksService_GetJwkSet_Error(t *testing.T) {
	repo := new(MockJwksRepo)
	svc := newTestJwksService(repo)

	repo.On("GetVersion", mock.Anything).Return(nil, errors.New("db down")).Once()

	set, err := svc.GetJwkSet(context.Background())

	assert.Nil(t, set)
	assert.EqualError(t, err, "db down")
	repo.AssertExpectations(t)
}

func TestJwksService_Rotate_Success(t *testing.T) {
	repo := new(MockJwksRepo)
	svc := newTestJwksService(repo)

	repo.On("Rotate", mock.Anything, mock.MatchedBy(func(k domain.SigningKeyEntity) bool {
		return k.Status == domain.Active && k.Kid != "" && k.PublicKey != "" && k.PrivateKey != ""
	})).Return(nil).Once()

	err := svc.Rotate(context.Background())

	assert.NoError(t, err)
	repo.AssertExpectations(t)
}

func TestJwksService_Rotate_RepoError(t *testing.T) {
	repo := new(MockJwksRepo)
	svc := newTestJwksService(repo)

	repo.On("Rotate", mock.Anything, mock.Anything).Return(errors.New("db down")).Once()

	err := svc.Rotate(context.Background())

	assert.EqualError(t, err, "db down")
	repo.AssertExpectations(t)
}

func TestJwksService_GetActiveKey_Success(t *testing.T) {
	repo := new(MockJwksRepo)
	svc := newTestJwksService(repo)

	now := time.Now()
	future := now.Add(24 * time.Hour)
	entity := makeKeyEntity(t, "kid1", domain.Active, future)

	repo.On("GetActiveKey", mock.Anything).Return(&entity, nil).Once()

	key, err := svc.GetActiveKey(context.Background())

	assert.NoError(t, err)
	assert.NotNil(t, key)
	assert.Equal(t, "kid1", key.Kid)
	assert.NotNil(t, key.PrivateKey)
	assert.NotNil(t, key.PublicKey)
	repo.AssertExpectations(t)
}

func TestJwksService_GetActiveKey_Error(t *testing.T) {
	repo := new(MockJwksRepo)
	svc := newTestJwksService(repo)

	repo.On("GetActiveKey", mock.Anything).Return(nil, errors.New("no active key")).Once()

	key, err := svc.GetActiveKey(context.Background())

	assert.Nil(t, key)
	assert.EqualError(t, err, "no active key")
	repo.AssertExpectations(t)
}

func TestJwksService_GetPublicKey_Success(t *testing.T) {
	repo := new(MockJwksRepo)
	svc := newTestJwksService(repo)

	now := time.Now()
	future := now.Add(24 * time.Hour)
	entities := []domain.SigningKeyEntity{makeKeyEntity(t, "kid1", domain.Active, future)}

	repo.On("GetVersion", mock.Anything).Return(&now, nil).Once()
	repo.On("Load", mock.Anything).Return(entities, nil).Once()

	pub, err := svc.GetPublicKey(context.Background(), "kid1")

	assert.NoError(t, err)
	assert.NotNil(t, pub)
	repo.AssertExpectations(t)
}

func TestJwksService_GetPublicKey_NoKey(t *testing.T) {
	repo := new(MockJwksRepo)
	svc := newTestJwksService(repo)

	now := time.Now()
	future := now.Add(24 * time.Hour)
	entities := []domain.SigningKeyEntity{makeKeyEntity(t, "kid1", domain.Active, future)}

	repo.On("GetVersion", mock.Anything).Return(&now, nil).Once()
	repo.On("Load", mock.Anything).Return(entities, nil).Once()

	pub, err := svc.GetPublicKey(context.Background(), "unknown-kid")

	assert.Nil(t, pub)
	assert.ErrorIs(t, err, errs.ErrNoKey)
	repo.AssertExpectations(t)
}

func TestJwksService_GetPublicKey_LoadError(t *testing.T) {
	repo := new(MockJwksRepo)
	svc := newTestJwksService(repo)

	repo.On("GetVersion", mock.Anything).Return(nil, errors.New("db down")).Once()

	pub, err := svc.GetPublicKey(context.Background(), "kid1")

	assert.Nil(t, pub)
	assert.EqualError(t, err, "db down")
	repo.AssertExpectations(t)
}
