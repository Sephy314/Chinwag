package service

import (
	"context"
	"crypto/ecdsa"
	"time"

	"github.com/Sephy314/chinwag/backend/services/auth/domain"
	"github.com/Sephy314/chinwag/backend/services/auth/repo"
	"github.com/Sephy314/chinwag/backend/services/auth/shared/cache"
	"github.com/Sephy314/chinwag/backend/services/auth/shared/logger"
	"github.com/Sephy314/chinwag/backend/services/auth/structs"
	"github.com/lestrrat-go/jwx/v3/jwk"
	"github.com/stretchr/testify/mock"
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

func (m *MockUserRepo) GetUserIncludingDeleted(ctx context.Context, id string) (*domain.User, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.User), args.Error(1)
}

func (m *MockUserRepo) RestoreUser(ctx context.Context, id string) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockUserRepo) SetRole(ctx context.Context, id string, role domain.Role) error {
	args := m.Called(ctx, id, role)
	return args.Error(0)
}

func (m *MockUserRepo) CountUsers(ctx context.Context) (int, error) {
	args := m.Called(ctx)
	return args.Int(0), args.Error(1)
}

func (m *MockUserRepo) CountAdmins(ctx context.Context) (int, error) {
	args := m.Called(ctx)
	return args.Int(0), args.Error(1)
}

func (m *MockUserRepo) ListUsers(ctx context.Context, cursor string, limit int, role, deleted, search string) ([]domain.User, *structs.CursorMeta, error) {
	args := m.Called(ctx, cursor, limit, role, deleted, search)
	if args.Get(0) == nil {
		return nil, nil, args.Error(2)
	}
	var meta *structs.CursorMeta
	if args.Get(1) != nil {
		meta = args.Get(1).(*structs.CursorMeta)
	}
	return args.Get(0).([]domain.User), meta, args.Error(2)
}

// ---- JWKS repo ----

type MockJwksRepo struct {
	mock.Mock
}

func (m *MockJwksRepo) Load(ctx context.Context) ([]domain.SigningKeyEntity, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]domain.SigningKeyEntity), args.Error(1)
}

func (m *MockJwksRepo) Rotate(ctx context.Context, key domain.SigningKeyEntity) error {
	args := m.Called(ctx, key)
	return args.Error(0)
}

func (m *MockJwksRepo) InActiveKey(ctx context.Context, kid string) error {
	args := m.Called(ctx, kid)
	return args.Error(0)
}

func (m *MockJwksRepo) ExpireActiveKey(ctx context.Context, keyType domain.KeyType) error {
	args := m.Called(ctx, keyType)
	return args.Error(0)
}

func (m *MockJwksRepo) ClearRetiredKeys(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *MockJwksRepo) GetActiveKey(ctx context.Context, keyType domain.KeyType) (*domain.SigningKeyEntity, error) {
	args := m.Called(ctx, keyType)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.SigningKeyEntity), args.Error(1)
}

func (m *MockJwksRepo) GetKeyByKid(ctx context.Context, kid string) (*domain.SigningKeyEntity, error) {
	args := m.Called(ctx, kid)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.SigningKeyEntity), args.Error(1)
}

func (m *MockJwksRepo) GetVersion(ctx context.Context) (*time.Time, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*time.Time), args.Error(1)
}

// ---- Logger ----

// MockLogger is a no-op logger that satisfies logger.Logger. Services log on
// many paths; these unit tests focus on business logic, not log output.
type MockLogger struct{}

func (MockLogger) Info(msg string, args ...any)  {}
func (MockLogger) Error(msg string, args ...any) {}
func (MockLogger) Debug(msg string, args ...any) {}
func (MockLogger) Warn(msg string, args ...any)  {}
func (MockLogger) Fatal(msg string, args ...any) {}
func (l MockLogger) With(args ...any) logger.Logger {
	return l
}

// ---- Jwks service (for UserService / JwtService) ----

type MockJwksService struct {
	mock.Mock
}

func (m *MockJwksService) LoadJWKS(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *MockJwksService) GetJwkSet(ctx context.Context) (jwk.Set, error) {
	panic("not implemented in this mock")
}

func (m *MockJwksService) RotateAccess(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *MockJwksService) RotateRefresh(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *MockJwksService) GetActiveAccessKey(ctx context.Context) (*domain.SigningKey, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.SigningKey), args.Error(1)
}

func (m *MockJwksService) GetActiveRefreshKey(ctx context.Context) (*domain.SigningKey, error) {
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

func (m *MockJwksService) GetRefreshKeyByKid(ctx context.Context, kid string) (*domain.SigningKey, error) {
	args := m.Called(ctx, kid)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.SigningKey), args.Error(1)
}

// ---- Refresh token service (for UserService / JwtService) ----

type MockRefreshTokenService struct {
	mock.Mock
}

func (m *MockRefreshTokenService) ValidateRefreshToken(ctx context.Context, rawToken, jkt string) (*structs.RefreshTokenClaims, error) {
	args := m.Called(ctx, rawToken, jkt)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*structs.RefreshTokenClaims), args.Error(1)
}

func (m *MockRefreshTokenService) RotateRefreshToken(ctx context.Context, claims *structs.RefreshTokenClaims) (*structs.RotatedRefreshToken, error) {
	args := m.Called(ctx, claims)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*structs.RotatedRefreshToken), args.Error(1)
}

func (m *MockRefreshTokenService) IssueRefreshToken(ctx context.Context, subject, jkt, sid string) (string, error) {
	args := m.Called(ctx, subject, jkt, sid)
	return args.String(0), args.Error(1)
}

func (m *MockRefreshTokenService) RevokeLineage(ctx context.Context, lineageID string) error {
	args := m.Called(ctx, lineageID)
	return args.Error(0)
}

// ---- Auth unit of work ----

type testTransaction struct {
	userRepo repo.UserRepository
	jwtRepo  repo.JwksRepository
}

func (t *testTransaction) UserRepo() repo.UserRepository { return t.userRepo }
func (t *testTransaction) JwtRepo() repo.JwksRepository  { return t.jwtRepo }

type testUnitOfWork struct {
	userRepo repo.UserRepository
	jwtRepo  repo.JwksRepository
}

func (u *testUnitOfWork) Do(ctx context.Context, fn func(ctx context.Context, tx repo.Transaction) error) error {
	return fn(ctx, &testTransaction{userRepo: u.userRepo, jwtRepo: u.jwtRepo})
}

func makeUow(userRepo repo.UserRepository, jwtRepo repo.JwksRepository) repo.UnitOfWork {
	return &testUnitOfWork{userRepo: userRepo, jwtRepo: jwtRepo}
}

// ---- Auth cache ----

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

func (m *MockCache) SRem(ctx context.Context, key string, members ...string) error {
	args := m.Called(ctx, key, members)
	return args.Error(0)
}

func (m *MockCache) ZAdd(ctx context.Context, key string, score int64, member string) error {
	args := m.Called(ctx, key, score, member)
	return args.Error(0)
}

func (m *MockCache) ZRem(ctx context.Context, key string, members ...string) error {
	args := m.Called(ctx, key, members)
	return args.Error(0)
}

func (m *MockCache) ZCard(ctx context.Context, key string) (int64, error) {
	args := m.Called(ctx, key)
	return args.Get(0).(int64), args.Error(1)
}

func (m *MockCache) ZRangeByScore(ctx context.Context, key, min, max string, offset, count int64) ([]string, error) {
	args := m.Called(ctx, key, min, max, offset, count)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]string), args.Error(1)
}

func (m *MockCache) ZRevRangeByScore(ctx context.Context, key, max, min string, offset, count int64) ([]string, error) {
	args := m.Called(ctx, key, max, min, offset, count)
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

// ensure MockCache satisfies cache.Cache
var _ cache.Cache = (*MockCache)(nil)
