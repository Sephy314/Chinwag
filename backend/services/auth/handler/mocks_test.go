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
	sharedauth "github.com/Sephy314/chinwag/backend/shared/auth"
	"github.com/Sephy314/chinwag/backend/shared/auth/dpop"
	"github.com/golang-jwt/jwt/v5"
	"github.com/labstack/echo/v5"
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

// ---- Refresh token service ----

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

// ---- Audit repo ----

type MockAuditRepo struct {
	mock.Mock
}

func (m *MockAuditRepo) Insert(ctx context.Context, ev domain.AuditEvent) error {
	args := m.Called(ctx, ev)
	return args.Error(0)
}

func (m *MockAuditRepo) List(ctx context.Context, cursor string, limit int, adminID, action, targetType string) ([]domain.AuditEvent, *structs.CursorMeta, error) {
	args := m.Called(ctx, cursor, limit, adminID, action, targetType)
	if args.Get(0) == nil {
		return nil, nil, args.Error(2)
	}
	var meta *structs.CursorMeta
	if args.Get(1) != nil {
		meta = args.Get(1).(*structs.CursorMeta)
	}
	return args.Get(0).([]domain.AuditEvent), meta, args.Error(2)
}

// setAdminClaims sets the verified ADMIN claims expected by adminID().
func setAdminClaims(c *echo.Context) {
	claims := &sharedauth.Claims{Role: sharedauth.RoleAdmin, RegisteredClaims: jwt.RegisteredClaims{Subject: "admin1"}}
	c.Set(sharedauth.ClaimsContextKey, claims)
}

func newAdminUserHandler(userRepo repo.UserRepository, cache cache.Cache, audit repo.AuditRepoInterface) *AdminUserHandler {
	userSvc := service.NewUserService(userRepo, new(MockJwksService), new(MockRefreshTokenService), &noopLogger{})
	refreshSvc := service.NewRefreshTokenService(cache, new(MockJwksService), "refresh:", time.Hour)
	auditSvc := service.NewAuditService(audit)
	return NewAdminUserHandler(userSvc, refreshSvc, auditSvc, &noopLogger{})
}

func newAdminSessionHandler(cache cache.Cache, audit repo.AuditRepoInterface) *AdminSessionHandler {
	refreshSvc := service.NewRefreshTokenService(cache, new(MockJwksService), "refresh:", time.Hour)
	auditSvc := service.NewAuditService(audit)
	return NewAdminSessionHandler(refreshSvc, auditSvc, &noopLogger{})
}
