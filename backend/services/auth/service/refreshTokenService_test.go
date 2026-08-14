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
	"github.com/Sephy314/chinwag/backend/services/auth/structs"
	"github.com/lestrrat-go/jwx/v3/jwa"
	"github.com/lestrrat-go/jwx/v3/jws"
	jwxjwt "github.com/lestrrat-go/jwx/v3/jwt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func newRefreshSvc(cache *MockCache, jwks *MockJwksService) *RefreshTokenService {
	return NewRefreshTokenService(cache, jwks, "rt:", time.Hour)
}

func makeRefreshKey(t *testing.T) *domain.SigningKey {
	t.Helper()
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	return &domain.SigningKey{
		Kid:        "ref-kid",
		Type:       domain.KeyTypeRefresh,
		PublicKey:  &priv.PublicKey,
		PrivateKey: priv,
		Status:     domain.Active,
	}
}

// signedRT produces a refresh-token JWT signed by the given key.
func signedRT(t *testing.T, subject, jti, sid, jkt string, key *domain.SigningKey, now time.Time) string {
	t.Helper()
	tok, err := internaljwt.SignRefreshToken(subject, jti, sid, jkt, key.PrivateKey, key.Kid, now, time.Hour)
	require.NoError(t, err)
	return tok
}

func TestRefreshTokenService_ValidateRefreshToken_Success(t *testing.T) {
	cache := new(MockCache)
	jwks := new(MockJwksService)
	svc := newRefreshSvc(cache, jwks)

	key := makeRefreshKey(t)
	now := time.Now()
	raw := signedRT(t, "u1", "jti1", "sid1", "jkt-abc", key, now)

	jwks.On("GetRefreshKeyByKid", mock.Anything, "ref-kid").Return(key, nil).Once()

	claims, err := svc.ValidateRefreshToken(context.Background(), raw, "jkt-abc")

	assert.NoError(t, err)
	assert.NotNil(t, claims)
	assert.Equal(t, "u1", claims.Subject)
	assert.Equal(t, "jti1", claims.JTI)
	assert.Equal(t, "sid1", claims.SID)
	assert.Equal(t, "jkt-abc", claims.Jkt)
	assert.Equal(t, now.Unix(), claims.IssuedAt)
	jwks.AssertExpectations(t)
}

func TestRefreshTokenService_ValidateRefreshToken_Garbage(t *testing.T) {
	cache := new(MockCache)
	jwks := new(MockJwksService)
	svc := newRefreshSvc(cache, jwks)

	claims, err := svc.ValidateRefreshToken(context.Background(), "not-a-jwt", "jkt")
	assert.Nil(t, claims)
	assert.ErrorIs(t, err, errs.ErrInvalidRefreshToken)
	jwks.AssertNotCalled(t, "GetRefreshKeyByKid", mock.Anything, mock.Anything)
}

func TestRefreshTokenService_ValidateRefreshToken_WrongKeyType(t *testing.T) {
	cache := new(MockCache)
	jwks := new(MockJwksService)
	svc := newRefreshSvc(cache, jwks)

	// An Access key signs the "refresh token" — must be rejected by key type.
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	accessKey := &domain.SigningKey{Kid: "acc-kid", Type: domain.KeyTypeAccess, PublicKey: &priv.PublicKey, PrivateKey: priv, Status: domain.Active}
	raw := signedRT(t, "u1", "jti1", "sid1", "jkt", accessKey, time.Now())

	jwks.On("GetRefreshKeyByKid", mock.Anything, "acc-kid").Return(accessKey, nil).Once()

	claims, err := svc.ValidateRefreshToken(context.Background(), raw, "jkt")

	assert.Nil(t, claims)
	assert.ErrorIs(t, err, errs.ErrInvalidRefreshToken)
	jwks.AssertExpectations(t)
}

func TestRefreshTokenService_ValidateRefreshToken_BindingMismatch(t *testing.T) {
	cache := new(MockCache)
	jwks := new(MockJwksService)
	svc := newRefreshSvc(cache, jwks)

	key := makeRefreshKey(t)
	raw := signedRT(t, "u1", "jti1", "sid1", "jkt-bound", key, time.Now())

	jwks.On("GetRefreshKeyByKid", mock.Anything, "ref-kid").Return(key, nil).Once()

	claims, err := svc.ValidateRefreshToken(context.Background(), raw, "jkt-other")

	assert.Nil(t, claims)
	assert.ErrorIs(t, err, errs.ErrRefreshTokenBindingMismatch)
	jwks.AssertExpectations(t)
}

func TestRefreshTokenService_ValidateRefreshToken_KeyLookupError(t *testing.T) {
	cache := new(MockCache)
	jwks := new(MockJwksService)
	svc := newRefreshSvc(cache, jwks)

	key := makeRefreshKey(t)
	raw := signedRT(t, "u1", "jti1", "sid1", "jkt", key, time.Now())

	jwks.On("GetRefreshKeyByKid", mock.Anything, "ref-kid").Return(nil, errors.New("no key")).Once()

	claims, err := svc.ValidateRefreshToken(context.Background(), raw, "jkt")

	assert.Nil(t, claims)
	assert.ErrorIs(t, err, errs.ErrInvalidRefreshToken)
	jwks.AssertExpectations(t)
}

// Defense in depth: even if some future issuer path produces a signed RT with
// no cnf claim, validation must reject it rather than treat it as valid.
func TestRefreshTokenService_ValidateRefreshToken_MissingCNFRejected(t *testing.T) {
	cache := new(MockCache)
	jwks := new(MockJwksService)
	svc := newRefreshSvc(cache, jwks)

	key := makeRefreshKey(t)
	// Build a valid signed RT WITHOUT the cnf claim directly (bypassing
	// SignRefreshToken, which already forbids empty jkt).
	now := time.Now()
	token, err := jwxjwt.NewBuilder().
		Issuer(internaljwt.RefreshTokenIssuer).
		Subject("u1").
		Audience([]string{internaljwt.RefreshTokenAudience}).
		IssuedAt(now).
		Expiration(now.Add(time.Hour)).
		JwtID("jti1").
		Claim("sid", "sid1").
		Build()
	require.NoError(t, err)
	headers := jws.NewHeaders()
	require.NoError(t, headers.Set("kid", "ref-kid"))
	raw, err := jwxjwt.Sign(token, jwxjwt.WithKey(jwa.ES256(), key.PrivateKey, jws.WithProtectedHeaders(headers)))
	require.NoError(t, err)

	jwks.On("GetRefreshKeyByKid", mock.Anything, "ref-kid").Return(key, nil).Once()

	claims, err := svc.ValidateRefreshToken(context.Background(), string(raw), "some-jkt")

	assert.Nil(t, claims)
	assert.ErrorIs(t, err, errs.ErrRefreshTokenBindingMismatch)
	jwks.AssertExpectations(t)
}

func TestRefreshTokenService_RotateRefreshToken_OK(t *testing.T) {
	cache := new(MockCache)
	jwks := new(MockJwksService)
	svc := newRefreshSvc(cache, jwks)

	key := makeRefreshKey(t)
	claims := &structs.RefreshTokenClaims{Subject: "u1", JTI: "old-jti", SID: "sid1", Jkt: "jkt"}

	jwks.On("GetActiveRefreshKey", mock.Anything).Return(key, nil).Once()
	cache.On("Eval", mock.Anything, mock.Anything, []string{"rt:old-jti", "rt:lineage:sid1"}, mock.Anything).Return([]interface{}{"OK", "sid1", "u1"}, nil).Once()

	rotated, err := svc.RotateRefreshToken(context.Background(), claims)

	assert.NoError(t, err)
	assert.NotNil(t, rotated)
	assert.Equal(t, "sid1", rotated.SID)
	assert.Equal(t, "u1", rotated.UserID)
	assert.NotEmpty(t, rotated.NewToken)
	assert.NotEmpty(t, rotated.NewJTI)
	// the new JWT must carry the new jti and the same sid
	verified, _, err := internaljwt.VerifyRefreshToken(rotated.NewToken, key.PublicKey, time.Now())
	assert.NoError(t, err)
	if assert.NotNil(t, verified) {
		assert.Equal(t, rotated.NewJTI, verified.JTI)
		assert.Equal(t, "sid1", verified.SID)
	}
	jwks.AssertExpectations(t)
	cache.AssertExpectations(t)
}

func TestRefreshTokenService_RotateRefreshToken_Reused(t *testing.T) {
	cache := new(MockCache)
	jwks := new(MockJwksService)
	svc := newRefreshSvc(cache, jwks)

	key := makeRefreshKey(t)
	claims := &structs.RefreshTokenClaims{Subject: "u1", JTI: "old-jti", SID: "sid1", Jkt: "jkt"}

	jwks.On("GetActiveRefreshKey", mock.Anything).Return(key, nil).Once()
	cache.On("Eval", mock.Anything, mock.Anything, []string{"rt:old-jti", "rt:lineage:sid1"}, mock.Anything).Return([]interface{}{"REUSED", "sid1", "u1"}, nil).Once()

	rotated, err := svc.RotateRefreshToken(context.Background(), claims)

	assert.Nil(t, rotated)
	assert.ErrorIs(t, err, errs.ErrRefreshTokenReused)
	jwks.AssertExpectations(t)
	cache.AssertExpectations(t)
}

func TestRefreshTokenService_RotateRefreshToken_Revoked(t *testing.T) {
	cache := new(MockCache)
	jwks := new(MockJwksService)
	svc := newRefreshSvc(cache, jwks)

	key := makeRefreshKey(t)
	claims := &structs.RefreshTokenClaims{Subject: "u1", JTI: "old-jti", SID: "sid1", Jkt: "jkt"}

	jwks.On("GetActiveRefreshKey", mock.Anything).Return(key, nil).Once()
	cache.On("Eval", mock.Anything, mock.Anything, []string{"rt:old-jti", "rt:lineage:sid1"}, mock.Anything).Return([]interface{}{"LINEAGE_REVOKED", "sid1", "u1"}, nil).Once()

	rotated, err := svc.RotateRefreshToken(context.Background(), claims)

	assert.Nil(t, rotated)
	assert.ErrorIs(t, err, errs.ErrRefreshTokenRevoked)
	jwks.AssertExpectations(t)
	cache.AssertExpectations(t)
}

func TestRefreshTokenService_RotateRefreshToken_NotFound(t *testing.T) {
	cache := new(MockCache)
	jwks := new(MockJwksService)
	svc := newRefreshSvc(cache, jwks)

	key := makeRefreshKey(t)
	claims := &structs.RefreshTokenClaims{Subject: "u1", JTI: "old-jti", SID: "sid1", Jkt: "jkt"}

	jwks.On("GetActiveRefreshKey", mock.Anything).Return(key, nil).Once()
	cache.On("Eval", mock.Anything, mock.Anything, []string{"rt:old-jti", "rt:lineage:sid1"}, mock.Anything).Return([]interface{}{"NOT_FOUND", "", ""}, nil).Once()

	rotated, err := svc.RotateRefreshToken(context.Background(), claims)

	assert.Nil(t, rotated)
	assert.ErrorIs(t, err, errs.ErrInvalidGrant)
	jwks.AssertExpectations(t)
	cache.AssertExpectations(t)
}

func TestRefreshTokenService_RotateRefreshToken_DependencyError(t *testing.T) {
	cache := new(MockCache)
	jwks := new(MockJwksService)
	svc := newRefreshSvc(cache, jwks)

	key := makeRefreshKey(t)
	claims := &structs.RefreshTokenClaims{Subject: "u1", JTI: "old-jti", SID: "sid1", Jkt: "jkt"}

	jwks.On("GetActiveRefreshKey", mock.Anything).Return(key, nil).Once()
	cache.On("Eval", mock.Anything, mock.Anything, []string{"rt:old-jti", "rt:lineage:sid1"}, mock.Anything).Return(nil, errors.New("redis down")).Once()

	rotated, err := svc.RotateRefreshToken(context.Background(), claims)

	assert.Nil(t, rotated)
	assert.ErrorIs(t, err, errs.ErrDependencyUnavailable)
	jwks.AssertExpectations(t)
	cache.AssertExpectations(t)
}

func TestRefreshTokenService_RotateRefreshToken_Timeout(t *testing.T) {
	cache := new(MockCache)
	jwks := new(MockJwksService)
	svc := newRefreshSvc(cache, jwks)

	key := makeRefreshKey(t)
	claims := &structs.RefreshTokenClaims{Subject: "u1", JTI: "old-jti", SID: "sid1", Jkt: "jkt"}

	jwks.On("GetActiveRefreshKey", mock.Anything).Return(key, nil).Once()
	cache.On("Eval", mock.Anything, mock.Anything, []string{"rt:old-jti", "rt:lineage:sid1"}, mock.Anything).Return(nil, context.DeadlineExceeded).Once()

	rotated, err := svc.RotateRefreshToken(context.Background(), claims)

	assert.Nil(t, rotated)
	assert.ErrorIs(t, err, errs.ErrDependencyTimeout)
	jwks.AssertExpectations(t)
	cache.AssertExpectations(t)
}

func TestRefreshTokenService_RotateRefreshToken_SigningFailure(t *testing.T) {
	cache := new(MockCache)
	jwks := new(MockJwksService)
	svc := newRefreshSvc(cache, jwks)

	claims := &structs.RefreshTokenClaims{Subject: "u1", JTI: "old-jti", SID: "sid1", Jkt: "jkt"}
	jwks.On("GetActiveRefreshKey", mock.Anything).Return(nil, errors.New("no active refresh key")).Once()

	rotated, err := svc.RotateRefreshToken(context.Background(), claims)

	assert.Nil(t, rotated)
	assert.ErrorIs(t, err, errs.ErrRotationFailed)
	cache.AssertNotCalled(t, "Eval", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
	jwks.AssertExpectations(t)
}

// Two concurrent refreshes of the SAME RT must never both succeed. The atomic
// Lua rotation script guarantees this in Redis; here we simulate the script's
// outcome (first rotation commits with OK, the racing one observes the token
// already consumed and is flagged REUSED) and assert exactly one success.
func TestRefreshTokenService_RotateRefreshToken_ConcurrentSingleSuccess(t *testing.T) {
	cache := new(MockCache)
	jwks := new(MockJwksService)
	svc := newRefreshSvc(cache, jwks)

	key := makeRefreshKey(t)
	claims := &structs.RefreshTokenClaims{Subject: "u1", JTI: "same-jti", SID: "sid1", Jkt: "jkt"}

	jwks.On("GetActiveRefreshKey", mock.Anything).Return(key, nil).Twice()
	// First rotation on the jti commits; the second (racing) call sees
	// status == 'consumed' and is reported as reuse.
	cache.On("Eval", mock.Anything, mock.Anything, []string{"rt:same-jti", "rt:lineage:sid1"}, mock.Anything).
		Return([]interface{}{"OK", "sid1", "u1"}, nil).Once()
	cache.On("Eval", mock.Anything, mock.Anything, []string{"rt:same-jti", "rt:lineage:sid1"}, mock.Anything).
		Return([]interface{}{"REUSED", "sid1", "u1"}, nil).Once()

	const n = 2
	results := make(chan error, n)
	for i := 0; i < n; i++ {
		go func() {
			_, err := svc.RotateRefreshToken(context.Background(), claims)
			results <- err
		}()
	}

	successes := 0
	reuses := 0
	for i := 0; i < n; i++ {
		err := <-results
		switch {
		case err == nil:
			successes++
		case errors.Is(err, errs.ErrRefreshTokenReused):
			reuses++
		default:
			t.Fatalf("unexpected error: %v", err)
		}
	}
	assert.Equal(t, 1, successes, "at most one concurrent rotation may succeed")
	assert.Equal(t, 1, reuses)
	jwks.AssertExpectations(t)
	cache.AssertExpectations(t)
}

func TestRefreshTokenService_IssueRefreshToken_Success(t *testing.T) {
	cache := new(MockCache)
	jwks := new(MockJwksService)
	svc := newRefreshSvc(cache, jwks)

	key := makeRefreshKey(t)
	jwks.On("GetActiveRefreshKey", mock.Anything).Return(key, nil).Once()
	cache.On("Eval", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return("OK", nil).Once()

	token, err := svc.IssueRefreshToken(context.Background(), "u1", "jkt", "")

	assert.NoError(t, err)
	assert.NotEmpty(t, token)
	jwks.AssertExpectations(t)
	cache.AssertExpectations(t)
}

func TestRefreshTokenService_IssueRefreshToken_DependencyError(t *testing.T) {
	cache := new(MockCache)
	jwks := new(MockJwksService)
	svc := newRefreshSvc(cache, jwks)

	key := makeRefreshKey(t)
	jwks.On("GetActiveRefreshKey", mock.Anything).Return(key, nil).Once()
	cache.On("Eval", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil, errors.New("redis down")).Once()

	token, err := svc.IssueRefreshToken(context.Background(), "u1", "jkt", "")

	assert.Empty(t, token)
	assert.ErrorIs(t, err, errs.ErrDependencyUnavailable)
	jwks.AssertExpectations(t)
	cache.AssertExpectations(t)
}

func TestRefreshTokenService_RevokeLineage_Empty(t *testing.T) {
	cache := new(MockCache)
	jwks := new(MockJwksService)
	svc := newRefreshSvc(cache, jwks)

	err := svc.RevokeLineage(context.Background(), "")

	assert.NoError(t, err)
	cache.AssertNotCalled(t, "SMembers", mock.Anything, mock.Anything)
}

func TestRefreshTokenService_RevokeLineage_Success(t *testing.T) {
	cache := new(MockCache)
	jwks := new(MockJwksService)
	svc := newRefreshSvc(cache, jwks)

	cache.On("HSet", mock.Anything, "rt:lineage:lin1", map[string]string{"status": "revoked"}, time.Duration(0)).Return(nil).Once()
	cache.On("SMembers", mock.Anything, "rt:lineage:lin1:members").Return([]string{"tok1", "tok2"}, nil).Once()
	cache.On("HSet", mock.Anything, "rt:tok1", map[string]string{"status": "revoked"}, time.Duration(0)).Return(nil).Once()
	cache.On("HSet", mock.Anything, "rt:tok2", map[string]string{"status": "revoked"}, time.Duration(0)).Return(nil).Once()
	cache.On("HGetAll", mock.Anything, "rt:tok1").Return(map[string]string{"user_id": "u1", "status": "active", "sid": "lin1"}, nil).Once()
	cache.On("ZRem", mock.Anything, "rt:user:u1", []string{"lin1"}).Return(nil).Once()
	cache.On("ZRem", mock.Anything, "rt:sessions", []string{"lin1"}).Return(nil).Once()

	err := svc.RevokeLineage(context.Background(), "lin1")

	assert.NoError(t, err)
	cache.AssertExpectations(t)
}

func TestRefreshTokenService_RevokeLineage_SMembersError(t *testing.T) {
	cache := new(MockCache)
	jwks := new(MockJwksService)
	svc := newRefreshSvc(cache, jwks)

	cache.On("HSet", mock.Anything, "rt:lineage:lin1", mock.Anything, time.Duration(0)).Return(nil).Once()
	cache.On("SMembers", mock.Anything, "rt:lineage:lin1:members").Return(nil, errors.New("redis down")).Once()

	err := svc.RevokeLineage(context.Background(), "lin1")

	assert.EqualError(t, err, "redis down")
	cache.AssertExpectations(t)
}

func TestHashRefreshToken_Stable(t *testing.T) {
	h1 := HashRefreshToken("abc")
	h2 := HashRefreshToken("abc")
	h3 := HashRefreshToken("abd")

	assert.Equal(t, h1, h2)
	assert.NotEqual(t, h1, h3)
	assert.Len(t, h1, 64)
}

func TestRefreshTokenService_ListSessionsByUser(t *testing.T) {
	cache := new(MockCache)
	jwks := new(MockJwksService)
	svc := newRefreshSvc(cache, jwks)

	cache.On("ZRevRangeByScore", mock.Anything, "rt:user:u1", "+inf", "-inf", int64(0), int64(-1)).Return([]string{"lin1"}, nil).Once()
	cache.On("SMembers", mock.Anything, "rt:lineage:lin1:members").Return([]string{"tok1"}, nil).Once()
	cache.On("HGetAll", mock.Anything, "rt:tok1").Return(map[string]string{
		"user_id": "u1", "sid": "lin1", "jkt": "jkt", "status": "active", "created_at": "1700000000",
	}, nil).Once()

	sessions, err := svc.ListSessionsByUser(context.Background(), "u1")

	assert.NoError(t, err)
	assert.Len(t, sessions, 1)
	assert.Equal(t, "lin1", sessions[0].LineageId)
	cache.AssertExpectations(t)
}

func TestRefreshTokenService_ListAllSessions_Paginated(t *testing.T) {
	cache := new(MockCache)
	jwks := new(MockJwksService)
	svc := newRefreshSvc(cache, jwks)

	// limit 2 -> fetch 3; hasMore true; keep first 2 (l1, l2)
	cache.On("ZRevRangeByScore", mock.Anything, "rt:sessions", "+inf", "-inf", int64(0), int64(3)).Return([]string{"l1", "l2", "l3"}, nil).Once()
	for _, lg := range []string{"l1", "l2"} {
		cache.On("SMembers", mock.Anything, "rt:lineage:"+lg+":members").Return([]string{"tok_" + lg}, nil).Once()
		cache.On("HGetAll", mock.Anything, "rt:tok_"+lg).Return(map[string]string{
			"user_id": "u1", "sid": lg, "status": "active", "created_at": "1700000000",
		}, nil).Once()
	}

	sessions, meta, err := svc.ListAllSessions(context.Background(), "", 2)

	assert.NoError(t, err)
	assert.Len(t, sessions, 2)
	assert.True(t, meta.HasMore)
	assert.NotEmpty(t, meta.NextCursor)
	cache.AssertExpectations(t)
}

func TestRefreshTokenService_CountSessions(t *testing.T) {
	cache := new(MockCache)
	jwks := new(MockJwksService)
	svc := newRefreshSvc(cache, jwks)

	cache.On("ZCard", mock.Anything, "rt:sessions").Return(int64(4), nil).Once()

	n, err := svc.CountSessions(context.Background())

	assert.NoError(t, err)
	assert.Equal(t, int64(4), n)
	cache.AssertExpectations(t)
}
