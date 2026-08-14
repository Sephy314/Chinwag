package service

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"database/sql"
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

func makeKeyEntityOfType(t *testing.T, kid string, kt domain.KeyType, status domain.KeyStatus, expiry time.Time) domain.SigningKeyEntity {
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
		Type:       kt,
		PublicKey:  pubB64,
		PrivateKey: privB64,
		Status:     status,
		CreatedAt:  now,
		UpdatedAt:  &now,
		ExpiredAt:  &expiry,
	}
}

func makeKeyEntity(t *testing.T, kid string, status domain.KeyStatus, expiry time.Time) domain.SigningKeyEntity {
	t.Helper()
	return makeKeyEntityOfType(t, kid, domain.KeyTypeAccess, status, expiry)
}

func newTestJwksService(repo *MockJwksRepo) *JwksService {
	return &JwksService{
		jwkSet: jwk.NewSet(),
		repo:   repo,
		log:    &MockLogger{},
	}
}

// accessRefreshEntities returns one active Access and one active Refresh key.
func accessRefreshEntities(t *testing.T, now time.Time) []domain.SigningKeyEntity {
	t.Helper()
	future := now.Add(24 * time.Hour)
	return []domain.SigningKeyEntity{
		makeKeyEntityOfType(t, "acc-kid", domain.KeyTypeAccess, domain.Active, future),
		makeKeyEntityOfType(t, "ref-kid", domain.KeyTypeRefresh, domain.Active, future),
	}
}

func TestJwksService_LoadJWKS_FirstBoot_CreatesBothKeyTypes(t *testing.T) {
	repo := new(MockJwksRepo)
	svc := newTestJwksService(repo)

	repo.On("GetVersion", mock.Anything).Return(nil, nil).Once() // no version yet
	repo.On("GetActiveKey", mock.Anything, domain.KeyTypeAccess).Return(nil, sql.ErrNoRows).Once()
	repo.On("Rotate", mock.Anything, mock.MatchedBy(func(k domain.SigningKeyEntity) bool {
		return k.Type == domain.KeyTypeAccess && k.Status == domain.Active
	})).Return(nil).Once()
	repo.On("GetActiveKey", mock.Anything, domain.KeyTypeRefresh).Return(nil, sql.ErrNoRows).Once()
	repo.On("Rotate", mock.Anything, mock.MatchedBy(func(k domain.SigningKeyEntity) bool {
		return k.Type == domain.KeyTypeRefresh && k.Status == domain.Active
	})).Return(nil).Once()
	repo.On("GetVersion", mock.Anything).Return(nil, nil).Once() // still nil after seeding

	err := svc.LoadJWKS(context.Background())

	assert.NoError(t, err)
	repo.AssertExpectations(t)
}

func TestJwksService_LoadJWKS_ReloadsOnNewVersion(t *testing.T) {
	repo := new(MockJwksRepo)
	svc := newTestJwksService(repo)

	now := time.Now()
	entities := accessRefreshEntities(t, now)

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

func TestJwksService_LoadJWKS_ExpiredAccessKey_RotatesAccessOnly(t *testing.T) {
	repo := new(MockJwksRepo)
	svc := newTestJwksService(repo)

	now := time.Now()
	past := now.Add(-time.Hour)
	future := now.Add(24 * time.Hour)
	expiredAccess := makeKeyEntityOfType(t, "expired-acc", domain.KeyTypeAccess, domain.Active, past)
	freshRefresh := makeKeyEntityOfType(t, "ref-kid", domain.KeyTypeRefresh, domain.Active, future)
	replacement := makeKeyEntityOfType(t, "fresh-acc", domain.KeyTypeAccess, domain.Active, future)

	repo.On("GetVersion", mock.Anything).Return(&now, nil).Once()
	repo.On("Load", mock.Anything).Return([]domain.SigningKeyEntity{expiredAccess, freshRefresh}, nil).Once()
	repo.On("ExpireActiveKey", mock.Anything, domain.KeyTypeAccess).Return(nil).Once()
	repo.On("Rotate", mock.Anything, mock.MatchedBy(func(k domain.SigningKeyEntity) bool {
		return k.Type == domain.KeyTypeAccess && k.Status == domain.Active
	})).Return(nil).Once()
	repo.On("Load", mock.Anything).Return([]domain.SigningKeyEntity{replacement, freshRefresh}, nil).Once()

	err := svc.LoadJWKS(context.Background())

	assert.NoError(t, err)
	repo.AssertExpectations(t)
}

func TestJwksService_LoadJWKS_MissingRefreshKey_BackfillsRefresh(t *testing.T) {
	repo := new(MockJwksRepo)
	svc := newTestJwksService(repo)

	now := time.Now()
	future := now.Add(24 * time.Hour)
	access := makeKeyEntityOfType(t, "acc-kid", domain.KeyTypeAccess, domain.Active, future)
	refresh := makeKeyEntityOfType(t, "ref-kid", domain.KeyTypeRefresh, domain.Active, future)

	repo.On("GetVersion", mock.Anything).Return(&now, nil).Once()
	// First load only has an Access key (e.g. pre-migration DB).
	repo.On("Load", mock.Anything).Return([]domain.SigningKeyEntity{access}, nil).Once()
	repo.On("Rotate", mock.Anything, mock.MatchedBy(func(k domain.SigningKeyEntity) bool {
		return k.Type == domain.KeyTypeRefresh && k.Status == domain.Active
	})).Return(nil).Once()
	repo.On("Load", mock.Anything).Return([]domain.SigningKeyEntity{access, refresh}, nil).Once()

	err := svc.LoadJWKS(context.Background())

	assert.NoError(t, err)
	repo.AssertExpectations(t)
}

func TestJwksService_GetJwkSet_Success(t *testing.T) {
	repo := new(MockJwksRepo)
	svc := newTestJwksService(repo)

	now := time.Now()
	entities := accessRefreshEntities(t, now)

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

func TestJwksService_RotateAccess_Success(t *testing.T) {
	repo := new(MockJwksRepo)
	svc := newTestJwksService(repo)

	repo.On("Rotate", mock.Anything, mock.MatchedBy(func(k domain.SigningKeyEntity) bool {
		return k.Type == domain.KeyTypeAccess && k.Status == domain.Active && k.Kid != "" && k.PublicKey != "" && k.PrivateKey != ""
	})).Return(nil).Once()

	err := svc.RotateAccess(context.Background())

	assert.NoError(t, err)
	repo.AssertExpectations(t)
}

func TestJwksService_RotateAccess_RepoError(t *testing.T) {
	repo := new(MockJwksRepo)
	svc := newTestJwksService(repo)

	repo.On("Rotate", mock.Anything, mock.Anything).Return(errors.New("db down")).Once()

	err := svc.RotateAccess(context.Background())

	assert.EqualError(t, err, "db down")
	repo.AssertExpectations(t)
}

func TestJwksService_RotateRefresh_Success(t *testing.T) {
	repo := new(MockJwksRepo)
	svc := newTestJwksService(repo)

	repo.On("Rotate", mock.Anything, mock.MatchedBy(func(k domain.SigningKeyEntity) bool {
		return k.Type == domain.KeyTypeRefresh && k.Status == domain.Active && k.Kid != "" && k.PublicKey != "" && k.PrivateKey != ""
	})).Return(nil).Once()

	err := svc.RotateRefresh(context.Background())

	assert.NoError(t, err)
	repo.AssertExpectations(t)
}

func TestJwksService_RotateRefresh_RepoError(t *testing.T) {
	repo := new(MockJwksRepo)
	svc := newTestJwksService(repo)

	repo.On("Rotate", mock.Anything, mock.Anything).Return(errors.New("db down")).Once()

	err := svc.RotateRefresh(context.Background())

	assert.EqualError(t, err, "db down")
	repo.AssertExpectations(t)
}

func TestJwksService_GetActiveAccessKey_Success(t *testing.T) {
	repo := new(MockJwksRepo)
	svc := newTestJwksService(repo)

	now := time.Now()
	future := now.Add(24 * time.Hour)
	entity := makeKeyEntityOfType(t, "acc-kid", domain.KeyTypeAccess, domain.Active, future)

	repo.On("GetActiveKey", mock.Anything, domain.KeyTypeAccess).Return(&entity, nil).Once()

	key, err := svc.GetActiveAccessKey(context.Background())

	assert.NoError(t, err)
	assert.NotNil(t, key)
	assert.Equal(t, domain.KeyTypeAccess, key.Type)
	assert.NotNil(t, key.PrivateKey)
	assert.NotNil(t, key.PublicKey)
	repo.AssertExpectations(t)
}

func TestJwksService_GetActiveAccessKey_Error(t *testing.T) {
	repo := new(MockJwksRepo)
	svc := newTestJwksService(repo)

	repo.On("GetActiveKey", mock.Anything, domain.KeyTypeAccess).Return(nil, errors.New("no active key")).Once()

	key, err := svc.GetActiveAccessKey(context.Background())

	assert.Nil(t, key)
	assert.EqualError(t, err, "no active key")
	repo.AssertExpectations(t)
}

func TestJwksService_GetActiveRefreshKey_Success(t *testing.T) {
	repo := new(MockJwksRepo)
	svc := newTestJwksService(repo)

	now := time.Now()
	future := now.Add(24 * time.Hour)
	entity := makeKeyEntityOfType(t, "ref-kid", domain.KeyTypeRefresh, domain.Active, future)

	repo.On("GetActiveKey", mock.Anything, domain.KeyTypeRefresh).Return(&entity, nil).Once()

	key, err := svc.GetActiveRefreshKey(context.Background())

	assert.NoError(t, err)
	assert.NotNil(t, key)
	assert.Equal(t, domain.KeyTypeRefresh, key.Type)
	assert.NotNil(t, key.PrivateKey)
	repo.AssertExpectations(t)
}

func TestJwksService_GetActiveRefreshKey_Error(t *testing.T) {
	repo := new(MockJwksRepo)
	svc := newTestJwksService(repo)

	repo.On("GetActiveKey", mock.Anything, domain.KeyTypeRefresh).Return(nil, errors.New("no active key")).Once()

	key, err := svc.GetActiveRefreshKey(context.Background())

	assert.Nil(t, key)
	assert.EqualError(t, err, "no active key")
	repo.AssertExpectations(t)
}

func TestJwksService_GetRefreshKeyByKid_Success(t *testing.T) {
	repo := new(MockJwksRepo)
	svc := newTestJwksService(repo)

	now := time.Now()
	future := now.Add(24 * time.Hour)
	entity := makeKeyEntityOfType(t, "ref-kid", domain.KeyTypeRefresh, domain.Inactive, future)

	repo.On("GetKeyByKid", mock.Anything, "ref-kid").Return(&entity, nil).Once()

	key, err := svc.GetRefreshKeyByKid(context.Background(), "ref-kid")

	assert.NoError(t, err)
	assert.Equal(t, domain.KeyTypeRefresh, key.Type)
	repo.AssertExpectations(t)
}

func TestJwksService_GetRefreshKeyByKid_WrongType(t *testing.T) {
	repo := new(MockJwksRepo)
	svc := newTestJwksService(repo)

	now := time.Now()
	future := now.Add(24 * time.Hour)
	entity := makeKeyEntityOfType(t, "acc-kid", domain.KeyTypeAccess, domain.Active, future)

	repo.On("GetKeyByKid", mock.Anything, "acc-kid").Return(&entity, nil).Once()

	key, err := svc.GetRefreshKeyByKid(context.Background(), "acc-kid")

	assert.Nil(t, key)
	assert.ErrorIs(t, err, errs.ErrNoKey)
	repo.AssertExpectations(t)
}

func TestJwksService_GetRefreshKeyByKid_Error(t *testing.T) {
	repo := new(MockJwksRepo)
	svc := newTestJwksService(repo)

	repo.On("GetKeyByKid", mock.Anything, "unknown").Return(nil, sql.ErrNoRows).Once()

	key, err := svc.GetRefreshKeyByKid(context.Background(), "unknown")

	assert.Nil(t, key)
	assert.ErrorIs(t, err, sql.ErrNoRows)
	repo.AssertExpectations(t)
}

func TestJwksService_GetPublicKey_Success(t *testing.T) {
	repo := new(MockJwksRepo)
	svc := newTestJwksService(repo)

	now := time.Now()
	entities := accessRefreshEntities(t, now)

	repo.On("GetVersion", mock.Anything).Return(&now, nil).Once()
	repo.On("Load", mock.Anything).Return(entities, nil).Once()

	pub, err := svc.GetPublicKey(context.Background(), "acc-kid")

	assert.NoError(t, err)
	assert.NotNil(t, pub)
	repo.AssertExpectations(t)
}

func TestJwksService_GetPublicKey_NoKey(t *testing.T) {
	repo := new(MockJwksRepo)
	svc := newTestJwksService(repo)

	now := time.Now()
	entities := accessRefreshEntities(t, now)

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
