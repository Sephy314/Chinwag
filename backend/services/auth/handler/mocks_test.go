package handler

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"net/http"
	"testing"
	"time"

	"github.com/Sephy314/chinwag/backend/services/auth/domain"
	"github.com/Sephy314/chinwag/backend/services/auth/repo"
	"github.com/Sephy314/chinwag/backend/services/auth/service"
	"github.com/Sephy314/chinwag/backend/services/auth/shared/cache"
	"github.com/Sephy314/chinwag/backend/services/auth/shared/logger"
	"github.com/Sephy314/chinwag/backend/services/auth/structs"
	"github.com/Sephy314/chinwag/backend/shared/auth/dpop"
	"github.com/lestrrat-go/jwx/v3/jwk"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"
)

// ---- User repo ----

type MockUserRepo struct {
	mock.Mock
}

func (m *MockUserRepo) CreateUser(ctx context.Context, user domain.User) error {
	args := m.Called(ctx, user)
	return args.Error(0)
}

func (m *MockUserRepo) CreateOAuthUser(ctx context.Context, user domain.User) error {
	args := m.Called(ctx, user)
	return args.Error(0)
}

func (m *MockUserRepo) GetUser(ctx context.Context, id string) (*domain.User, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.User), args.Error(1)
}

func (m *MockUserRepo) UpdateUser(ctx context.Context, user domain.User) error {
	args := m.Called(ctx, user)
	return args.Error(0)
}

func (m *MockUserRepo) DeleteUser(ctx context.Context, id string) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockUserRepo) GetUserByEmail(ctx context.Context, email string) (*domain.User, error) {
	args := m.Called(ctx, email)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.User), args.Error(1)
}

// ---- JWKS service ----

type MockJwksService struct {
	mock.Mock
}

func (m *MockJwksService) LoadJWKS(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *MockJwksService) GetJwkSet(ctx context.Context) (jwk.Set, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(jwk.Set), args.Error(1)
}

func (m *MockJwksService) Rotate(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *MockJwksService) GetActiveKey(ctx context.Context) (*domain.SigningKey, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.SigningKey), args.Error(1)
}

func (m *MockJwksService) GetPublicKey(ctx context.Context, kid string) (*ecdsa.PublicKey, error) {
	args := m.Called(ctx, kid)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*ecdsa.PublicKey), args.Error(1)
}

// ---- Refresh token service ----

type MockRefreshTokenService struct {
	mock.Mock
}

func (m *MockRefreshTokenService) GetRefreshToken(ctx context.Context, refreshToken string) (*structs.RefreshTokenRecord, error) {
	args := m.Called(ctx, refreshToken)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*structs.RefreshTokenRecord), args.Error(1)
}

func (m *MockRefreshTokenService) InsertRefreshToken(ctx context.Context, token structs.RefreshToken) error {
	args := m.Called(ctx, token)
	return args.Error(0)
}

func (m *MockRefreshTokenService) ConsumeRefreshToken(ctx context.Context, refreshToken string) (*structs.RefreshTokenRecord, error) {
	args := m.Called(ctx, refreshToken)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*structs.RefreshTokenRecord), args.Error(1)
}

func (m *MockRefreshTokenService) RevokeLineage(ctx context.Context, lineageID string) error {
	args := m.Called(ctx, lineageID)
	return args.Error(0)
}

// ---- JWT service ----

type MockJwtService struct {
	mock.Mock
}

func (m *MockJwtService) NewAccessToken(ctx context.Context, userId string, role domain.Role, jkt string) (*string, error) {
	args := m.Called(ctx, userId, role, jkt)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*string), args.Error(1)
}

// ---- DPoP service ----

type MockDPoPService struct {
	mock.Mock
}

func (m *MockDPoPService) Validate(ctx context.Context, r *http.Request) (*dpop.Proof, string, error) {
	args := m.Called(ctx, r)
	if args.Get(0) == nil {
		return nil, args.String(1), args.Error(2)
	}
	return args.Get(0).(*dpop.Proof), args.String(1), args.Error(2)
}

func (m *MockDPoPService) Validator() *dpop.Validator {
	args := m.Called()
	if args.Get(0) == nil {
		return nil
	}
	return args.Get(0).(*dpop.Validator)
}

// ---- Auth cache (used as the refresh lock store) ----

type MockCache struct {
	mock.Mock
}

func (m *MockCache) Get(ctx context.Context, key string) (string, error) {
	args := m.Called(ctx, key)
	return args.String(0), args.Error(1)
}

func (m *MockCache) Set(ctx context.Context, key string, value any, ttl time.Duration) error {
	args := m.Called(ctx, key, value, ttl)
	return args.Error(0)
}

func (m *MockCache) SetNX(ctx context.Context, key string, value any, ttl time.Duration) (bool, error) {
	args := m.Called(ctx, key, value, ttl)
	return args.Bool(0), args.Error(1)
}

func (m *MockCache) Delete(ctx context.Context, key string) error {
	args := m.Called(ctx, key)
	return args.Error(0)
}

func (m *MockCache) TTL(ctx context.Context, key string) (time.Duration, error) {
	args := m.Called(ctx, key)
	return args.Get(0).(time.Duration), args.Error(1)
}

func (m *MockCache) HSet(ctx context.Context, key string, fields map[string]string, ttl time.Duration) error {
	args := m.Called(ctx, key, fields, ttl)
	return args.Error(0)
}

func (m *MockCache) HGetAll(ctx context.Context, key string) (map[string]string, error) {
	args := m.Called(ctx, key)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(map[string]string), args.Error(1)
}

func (m *MockCache) SAdd(ctx context.Context, key string, ttl time.Duration, members ...string) error {
	args := m.Called(ctx, key, ttl, members)
	return args.Error(0)
}

func (m *MockCache) SMembers(ctx context.Context, key string) ([]string, error) {
	args := m.Called(ctx, key)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]string), args.Error(1)
}

func (m *MockCache) Eval(ctx context.Context, script string, keys []string, args ...any) (any, error) {
	mArgs := m.Called(ctx, script, keys, args)
	return mArgs.Get(0), mArgs.Error(1)
}

func (m *MockCache) AcquireLock(ctx context.Context, key, token string, ttl time.Duration) (bool, error) {
	args := m.Called(ctx, key, token, ttl)
	return args.Bool(0), args.Error(1)
}

func (m *MockCache) ReleaseLock(ctx context.Context, key, token string) error {
	args := m.Called(ctx, key, token)
	return args.Error(0)
}

func (m *MockCache) ConsumeNonce(ctx context.Context, nonce string) (bool, error) {
	args := m.Called(ctx, nonce)
	return args.Bool(0), args.Error(1)
}

func (m *MockCache) ReserveJti(ctx context.Context, jti string, ttl time.Duration) (bool, error) {
	args := m.Called(ctx, jti, ttl)
	return args.Bool(0), args.Error(1)
}

var _ cache.Cache = (*MockCache)(nil)

// ---- helpers ----

// makeProof builds a minimal *dpop.Proof carrying a fresh P-256 key so the
// handler's proof.Thumbprint() call succeeds.
func makeProof(t *testing.T) *dpop.Proof {
	t.Helper()
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	return &dpop.Proof{Key: &priv.PublicKey}
}

// proofJkt computes the RFC 7638 thumbprint for the key inside proof.
func proofJkt(t *testing.T, proof *dpop.Proof) string {
	t.Helper()
	jkt, err := proof.Thumbprint()
	require.NoError(t, err)
	return jkt
}

// makeSigningKey returns a *domain.SigningKey with a fresh P-256 key pair.
func makeSigningKey(t *testing.T) *domain.SigningKey {
	t.Helper()
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	return &domain.SigningKey{
		Kid:        "test-kid",
		PublicKey:  &priv.PublicKey,
		PrivateKey: priv,
		Status:     domain.Active,
	}
}

// mustHash returns a bcrypt hash of pw, failing the test on error.
func mustHash(t *testing.T, pw string) string {
	t.Helper()
	h, err := bcrypt.GenerateFromPassword([]byte(pw), bcrypt.MinCost)
	require.NoError(t, err)
	return string(h)
}

func newUserHandlerWith(userRepo repo.UserRepository, jwk *MockJwksService, refresh *MockRefreshTokenService, dpopSvc *MockDPoPService) *UserHandler {
	svc := service.NewUserService(userRepo, jwk, refresh, &noopLogger{})
	return NewUserHandler(svc, &noopLogger{}, dpopSvc)
}

type noopLogger struct{}

func (noopLogger) Info(msg string, args ...any)  {}
func (noopLogger) Error(msg string, args ...any) {}
func (noopLogger) Debug(msg string, args ...any) {}
func (noopLogger) Warn(msg string, args ...any)  {}
func (noopLogger) Fatal(msg string, args ...any) {}
func (l noopLogger) With(args ...any) logger.Logger {
	return l
}
