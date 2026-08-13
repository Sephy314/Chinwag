package service

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strconv"
	"time"

	"github.com/Sephy314/chinwag/backend/services/auth/domain"
	"github.com/Sephy314/chinwag/backend/services/auth/internal/jwt"
	"github.com/Sephy314/chinwag/backend/services/auth/shared/cache"
	"github.com/Sephy314/chinwag/backend/services/auth/shared/errs"
	"github.com/Sephy314/chinwag/backend/services/auth/structs"
	"github.com/google/uuid"
	"github.com/lestrrat-go/jwx/v3/jwa"
	"github.com/lestrrat-go/jwx/v3/jws"
)

// rotateRefreshTokenScript performs the refresh rotation as ONE atomic Redis
// operation (RFC 9700): it validates the current RT's Redis state, marks it
// consumed, creates the next RT in the same lineage, and updates lineage/index
// state. Concurrent requests racing on the same RT can therefore never both
// succeed — the second one observes status == 'consumed' and is reported as a
// REUSE, which the caller turns into a full lineage revocation.
//
// The consumed RT keeps a full TTL so a replay of any rotated token stays
// detectable for the lifetime of its lineage.
//
// KEYS[1] = refresh:{old_jti}
// KEYS[2] = refresh:lineage:{sid}
// ARGV[1] = sid, ARGV[2] = new_jti, ARGV[3] = user_id, ARGV[4] = jkt
// ARGV[5] = created_at (unix seconds), ARGV[6] = ttl (seconds), ARGV[7] = prefix
//
// Returns a 3-element array: {status, sid, user_id} where status is
// OK | NOT_FOUND | REVOKED | LINEAGE_REVOKED | REUSED.
const rotateRefreshTokenScript = `
local function hgetall(key)
  local rec = redis.call('HGETALL', key)
  local t = {}
  for i = 1, #rec, 2 do t[rec[i]] = rec[i + 1] end
  return t
end

local cur = hgetall(KEYS[1])
if next(cur) == nil then
  return {'NOT_FOUND', '', ''}
end

if cur['status'] == 'revoked' then
  return {'REVOKED', cur['sid'], cur['user_id']}
end

if cur['status'] == 'consumed' then
  return {'REUSED', cur['sid'], cur['user_id']}
end

local lin = hgetall(KEYS[2])
if lin['status'] == 'revoked' then
  return {'LINEAGE_REVOKED', cur['sid'], cur['user_id']}
end

-- preserve the lineage's original creation time so session ordering is stable
local lineageCreatedAt = lin['created_at']
if lineageCreatedAt == nil or lineageCreatedAt == '' then
  lineageCreatedAt = cur['created_at']
end

-- invalidate the current RT (keep the record around for reuse detection)
redis.call('HSET', KEYS[1], 'status', 'consumed')
redis.call('PEXPIRE', KEYS[1], tonumber(ARGV[6]) * 1000)

-- create the next RT in the same lineage
local newKey = ARGV[7] .. ARGV[2]
redis.call('HSET', newKey,
  'sid', ARGV[1],
  'user_id', ARGV[3],
  'jkt', ARGV[4],
  'status', 'active',
  'created_at', ARGV[5])
redis.call('PEXPIRE', newKey, tonumber(ARGV[6]) * 1000)

-- lineage state
redis.call('HSET', KEYS[2],
  'sid', ARGV[1],
  'user_id', ARGV[3],
  'jkt', ARGV[4],
  'status', 'active',
  'created_at', lineageCreatedAt)
redis.call('PEXPIRE', KEYS[2], tonumber(ARGV[6]) * 1000)

-- lineage membership + session indexes
redis.call('SADD', ARGV[7] .. 'lineage:' .. ARGV[1] .. ':members', ARGV[2])
redis.call('EXPIRE', ARGV[7] .. 'lineage:' .. ARGV[1] .. ':members', tonumber(ARGV[6]))
redis.call('ZADD', ARGV[7] .. 'user:' .. ARGV[3], tonumber(lineageCreatedAt), ARGV[1])
redis.call('ZADD', ARGV[7] .. 'sessions', tonumber(lineageCreatedAt), ARGV[1])

return {'OK', cur['sid'], ARGV[3]}
`

// issueRefreshTokenScript atomically creates the initial RT state for a fresh
// login (a brand-new lineage).
//
// KEYS[1] = refresh:{jti}, KEYS[2] = refresh:lineage:{sid},
// KEYS[3] = refresh:lineage:{sid}:members, KEYS[4] = refresh:user:{uid},
// KEYS[5] = refresh:sessions
// ARGV[1] = sid, ARGV[2] = jti, ARGV[3] = user_id, ARGV[4] = jkt,
// ARGV[5] = created_at (unix seconds), ARGV[6] = ttl (seconds)
const issueRefreshTokenScript = `
redis.call('HSET', KEYS[1],
  'sid', ARGV[1], 'user_id', ARGV[3], 'jkt', ARGV[4],
  'status', 'active', 'created_at', ARGV[5])
redis.call('PEXPIRE', KEYS[1], tonumber(ARGV[6]) * 1000)
redis.call('HSET', KEYS[2],
  'sid', ARGV[1], 'user_id', ARGV[3], 'jkt', ARGV[4],
  'status', 'active', 'created_at', ARGV[5])
redis.call('PEXPIRE', KEYS[2], tonumber(ARGV[6]) * 1000)
redis.call('SADD', KEYS[3], ARGV[2])
redis.call('EXPIRE', KEYS[3], tonumber(ARGV[6]))
redis.call('ZADD', KEYS[4], tonumber(ARGV[5]), ARGV[1])
redis.call('ZADD', KEYS[5], tonumber(ARGV[5]), ARGV[1])
return 'OK'
`

type RefreshTokenServiceInterface interface {
	// ValidateRefreshToken performs ONLY cryptographic validation of the
	// refresh-token JWT (signature, alg, claims, signing-key type, DPoP
	// binding). It never touches Redis, so RT validation stays available during
	// a Redis outage.
	ValidateRefreshToken(ctx context.Context, rawToken, jkt string) (*structs.RefreshTokenClaims, error)
	// RotateRefreshToken signs a fresh RT for the same lineage and atomically
	// commits the rotation in Redis (consume old jti, activate new jti). It
	// returns dependency errors when Redis is unavailable — in that case NO
	// rotation state change happens.
	RotateRefreshToken(ctx context.Context, claims *structs.RefreshTokenClaims) (*structs.RotatedRefreshToken, error)
	// IssueRefreshToken signs and persists the initial RT for a new login.
	IssueRefreshToken(ctx context.Context, subject, jkt, sid string) (string, error)
	RevokeLineage(ctx context.Context, lineageID string) error
}

type RefreshTokenService struct {
	Cache              cache.Cache
	Jwks               JwksServiceInterface
	RefreshTokenPrefix string
	RefreshTokenTTL    time.Duration
}

func NewRefreshTokenService(cache cache.Cache, jwks JwksServiceInterface, prefix string, ttl time.Duration) *RefreshTokenService {
	return &RefreshTokenService{
		Cache:              cache,
		Jwks:               jwks,
		RefreshTokenPrefix: prefix,
		RefreshTokenTTL:    ttl,
	}
}

// --- key helpers ---

func (r *RefreshTokenService) tokenKey(jti string) string { return r.RefreshTokenPrefix + jti }
func (r *RefreshTokenService) lineageKey(sid string) string {
	return r.RefreshTokenPrefix + "lineage:" + sid
}
func (r *RefreshTokenService) lineageMembersKey(sid string) string {
	return r.RefreshTokenPrefix + "lineage:" + sid + ":members"
}
func (r *RefreshTokenService) userKey(userID string) string { return r.RefreshTokenPrefix + "user:" + userID }
func (r *RefreshTokenService) sessionsKey() string          { return r.RefreshTokenPrefix + "sessions" }

// --- RT cryptographic validation (Redis-free) ---

func (r *RefreshTokenService) ValidateRefreshToken(ctx context.Context, rawToken, jkt string) (*structs.RefreshTokenClaims, error) {
	msg, err := jws.Parse([]byte(rawToken))
	if err != nil {
		return nil, errs.ErrInvalidRefreshToken
	}
	sigs := msg.Signatures()
	if len(sigs) != 1 {
		return nil, errs.ErrInvalidRefreshToken
	}
	hdr := sigs[0].ProtectedHeaders()
	alg, ok := hdr.Algorithm()
	if !ok || alg != jwa.ES256() {
		return nil, errs.InvalidAlgErr
	}
	kid, _ := hdr.KeyID()
	if kid == "" {
		return nil, errs.ErrInvalidRefreshToken
	}

	key, err := r.Jwks.GetRefreshKeyByKid(ctx, kid)
	if err != nil {
		return nil, errs.ErrInvalidRefreshToken
	}
	if key.Type != domain.KeyTypeRefresh {
		return nil, errs.ErrInvalidRefreshToken
	}

	verified, _, err := jwt.VerifyRefreshToken(rawToken, key.PublicKey, time.Now())
	if err != nil {
		return nil, errs.ErrInvalidRefreshToken
	}

	if verified.Jkt != "" && verified.Jkt != jkt {
		return nil, errs.ErrRefreshTokenBindingMismatch
	}

	return &structs.RefreshTokenClaims{
		Subject:   verified.Subject,
		JTI:       verified.JTI,
		SID:       verified.SID,
		Jkt:       verified.Jkt,
		IssuedAt:  verified.IssuedAt.Unix(),
		ExpiresAt: verified.Expires.Unix(),
	}, nil
}

// --- rotation ---

func (r *RefreshTokenService) RotateRefreshToken(ctx context.Context, claims *structs.RefreshTokenClaims) (*structs.RotatedRefreshToken, error) {
	key, err := r.Jwks.GetActiveRefreshKey(ctx)
	if err != nil {
		return nil, errs.ErrRotationFailed
	}

	newJTI := uuid.Must(uuid.NewV7()).String()
	now := time.Now()
	newToken, err := jwt.SignRefreshToken(claims.Subject, newJTI, claims.SID, claims.Jkt, key.PrivateKey, key.Kid, now, r.RefreshTokenTTL)
	if err != nil {
		return nil, errs.ErrRotationFailed
	}

	result, err := r.Cache.Eval(ctx, rotateRefreshTokenScript,
		[]string{r.tokenKey(claims.JTI), r.lineageKey(claims.SID)},
		claims.SID, newJTI, claims.Subject, claims.Jkt, now.Unix(), int64(r.RefreshTokenTTL.Seconds()), r.RefreshTokenPrefix)
	if err != nil {
		return nil, classifyDependencyError(err)
	}

	arr, ok := result.([]interface{})
	if !ok || len(arr) < 3 {
		return nil, errs.ErrInvalidGrant
	}

	switch asString(arr[0]) {
	case "OK":
		return &structs.RotatedRefreshToken{
			NewToken: newToken,
			NewJTI:   newJTI,
			SID:      asString(arr[1]),
			UserID:   asString(arr[2]),
		}, nil
	case "NOT_FOUND":
		return nil, errs.ErrInvalidGrant
	case "REVOKED", "LINEAGE_REVOKED":
		return nil, errs.ErrRefreshTokenRevoked
	case "REUSED":
		return nil, errs.ErrRefreshTokenReused
	default:
		return nil, errs.ErrInvalidGrant
	}
}

// --- initial issuance (login / oauth) ---

func (r *RefreshTokenService) IssueRefreshToken(ctx context.Context, subject, jkt, sid string) (string, error) {
	key, err := r.Jwks.GetActiveRefreshKey(ctx)
	if err != nil {
		return "", errs.ErrRotationFailed
	}

	jti := uuid.Must(uuid.NewV7()).String()
	if sid == "" {
		sid = uuid.Must(uuid.NewV7()).String()
	}
	now := time.Now()
	token, err := jwt.SignRefreshToken(subject, jti, sid, jkt, key.PrivateKey, key.Kid, now, r.RefreshTokenTTL)
	if err != nil {
		return "", errs.ErrRotationFailed
	}

	_, err = r.Cache.Eval(ctx, issueRefreshTokenScript,
		[]string{r.tokenKey(jti), r.lineageKey(sid), r.lineageMembersKey(sid), r.userKey(subject), r.sessionsKey()},
		sid, jti, subject, jkt, now.Unix(), int64(r.RefreshTokenTTL.Seconds()))
	if err != nil {
		return "", classifyDependencyError(err)
	}
	return token, nil
}

// --- revocation ---

func (r *RefreshTokenService) RevokeLineage(ctx context.Context, lineageID string) error {
	if lineageID == "" {
		return nil
	}

	// The lineage key is kept (with status=revoked, its existing TTL) so that
	// any later refresh attempt is rejected as a revoked lineage.
	if err := r.Cache.HSet(ctx, r.lineageKey(lineageID), map[string]string{"status": "revoked"}, 0); err != nil {
		return err
	}

	members, err := r.Cache.SMembers(ctx, r.lineageMembersKey(lineageID))
	if err != nil {
		return err
	}

	for _, jti := range members {
		if err := r.Cache.HSet(ctx, r.tokenKey(jti), map[string]string{"status": "revoked"}, 0); err != nil {
			return err
		}
	}

	// Remove the lineage from the per-user and global session indexes.
	if len(members) > 0 {
		if rec, rerr := r.getRecord(ctx, members[0]); rerr == nil && rec.UserID != "" {
			_ = r.Cache.ZRem(ctx, r.userKey(rec.UserID), lineageID)
		}
	}
	_ = r.Cache.ZRem(ctx, r.sessionsKey(), lineageID)

	return nil
}

// classifyDependencyError maps a Redis access failure to a distinct error so a
// Redis outage is never reported as an invalid refresh token.
func classifyDependencyError(err error) error {
	if errors.Is(err, context.DeadlineExceeded) {
		return errs.ErrDependencyTimeout
	}
	return errs.ErrDependencyUnavailable
}

func asString(v any) string {
	s, _ := v.(string)
	return s
}

// HashRefreshToken returns the SHA-256 hex digest of a refresh-token value. It
// is used only for derived keys (e.g. the per-token refresh lock); the JWT
// itself is keyed by its jti claim.
func HashRefreshToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// --- Admin session introspection ---

func (r *RefreshTokenService) getRecord(ctx context.Context, jti string) (*structs.RefreshTokenRecord, error) {
	fields, err := r.Cache.HGetAll(ctx, r.tokenKey(jti))
	if err != nil {
		return nil, err
	}
	if len(fields) == 0 {
		return nil, errs.ErrCacheNotFound
	}
	return r.toRecord(fields), nil
}

func (r *RefreshTokenService) toRecord(fields map[string]string) *structs.RefreshTokenRecord {
	createdAt, _ := strconv.ParseInt(fields["created_at"], 10, 64)
	status := fields["status"]
	return &structs.RefreshTokenRecord{
		UserID:    fields["user_id"],
		LineageID: fields["sid"],
		Jkt:       fields["jkt"],
		Used:      status == "consumed",
		Revoked:   status == "revoked",
		CreatedAt: createdAt,
	}
}

// sessionFromLineage builds a safe AdminSession for a lineage (one login). The
// raw refresh token value is never included.
func (r *RefreshTokenService) sessionFromLineage(ctx context.Context, lineageID string) (*structs.AdminSession, error) {
	members, err := r.Cache.SMembers(ctx, r.lineageMembersKey(lineageID))
	if err != nil {
		return nil, err
	}
	if len(members) == 0 {
		return nil, nil
	}
	rec, err := r.getRecord(ctx, members[0])
	if err != nil {
		return nil, err
	}
	return &structs.AdminSession{
		LineageId: lineageID,
		UserId:    rec.UserID,
		CreatedAt: rec.CreatedAt,
		Used:      rec.Used,
		Revoked:   rec.Revoked,
		Jkt:       rec.Jkt,
		Tokens:    len(members),
	}, nil
}

type sessionCursor struct {
	CreatedAt int64  `json:"created_at"`
	LineageId string `json:"lineage_id"`
}

func encodeSessionCursor(createdAt int64, lineageID string) string {
	c := sessionCursor{CreatedAt: createdAt, LineageId: lineageID}
	b, _ := json.Marshal(c)
	return base64.URLEncoding.EncodeToString(b)
}

func decodeSessionCursor(s string) (sessionCursor, error) {
	var c sessionCursor
	b, err := base64.URLEncoding.DecodeString(s)
	if err != nil {
		return c, err
	}
	err = json.Unmarshal(b, &c)
	return c, err
}

// ListSessionsByUser returns all lineages (sessions) belonging to a user,
// newest first, using the per-user Sorted Set index.
func (r *RefreshTokenService) ListSessionsByUser(ctx context.Context, userId string) ([]structs.AdminSession, error) {
	lineages, err := r.Cache.ZRevRangeByScore(ctx, r.RefreshTokenPrefix+"user:"+userId, "+inf", "-inf", 0, -1)
	if err != nil {
		return nil, err
	}
	out := make([]structs.AdminSession, 0, len(lineages))
	for _, lg := range lineages {
		s, err := r.sessionFromLineage(ctx, lg)
		if err != nil || s == nil {
			continue
		}
		out = append(out, *s)
	}
	return out, nil
}

// ListAllSessions returns a cursor-paginated page of all active sessions
// (lineages), newest first. It uses the global `refresh:sessions` Sorted Set so
// only the requested range is fetched from Redis; the cursor is deterministic
// on (created_at, lineage_id) to disambiguate identical timestamps.
func (r *RefreshTokenService) ListAllSessions(ctx context.Context, cursorStr string, limit int) ([]structs.AdminSession, *structs.CursorMeta, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}

	key := r.RefreshTokenPrefix + "sessions"
	fetchLimit := int64(limit + 1)
	lineageIDs := []string{}

	if cursorStr == "" {
		ids, err := r.Cache.ZRevRangeByScore(ctx, key, "+inf", "-inf", 0, fetchLimit)
		if err != nil {
			return nil, nil, err
		}
		lineageIDs = ids
	} else {
		c, err := decodeSessionCursor(cursorStr)
		if err != nil {
			return nil, nil, err
		}
		// Strictly older sessions, newest first.
		ids, err := r.Cache.ZRevRangeByScore(ctx, key, "("+strconv.FormatInt(c.CreatedAt, 10), "-inf", 0, fetchLimit)
		if err != nil {
			return nil, nil, err
		}
		lineageIDs = append(lineageIDs, ids...)
		// Ties at the cursor score: members that sort before the cursor in the
		// ZSET (asc) order come after it in reverse order.
		ties, err := r.Cache.ZRangeByScore(ctx, key, strconv.FormatInt(c.CreatedAt, 10), strconv.FormatInt(c.CreatedAt, 10), 0, -1)
		if err != nil {
			return nil, nil, err
		}
		for i := len(ties) - 1; i >= 0; i-- {
			if ties[i] < c.LineageId {
				lineageIDs = append(lineageIDs, ties[i])
			}
		}
		if len(lineageIDs) > int(fetchLimit) {
			lineageIDs = lineageIDs[:fetchLimit]
		}
	}

	hasMore := len(lineageIDs) > limit
	if hasMore {
		lineageIDs = lineageIDs[:limit]
	}

	sessions := make([]structs.AdminSession, 0, len(lineageIDs))
	for _, lg := range lineageIDs {
		s, err := r.sessionFromLineage(ctx, lg)
		if err != nil || s == nil {
			continue
		}
		sessions = append(sessions, *s)
	}

	var meta *structs.CursorMeta
	if hasMore && len(sessions) > 0 {
		last := sessions[len(sessions)-1]
		meta = &structs.CursorMeta{
			NextCursor: encodeSessionCursor(last.CreatedAt, last.LineageId),
			HasMore:    true,
		}
	}

	return sessions, meta, nil
}

// GetLineage returns a single session (lineage) for inspection.
func (r *RefreshTokenService) GetLineage(ctx context.Context, lineageID string) (*structs.AdminSession, error) {
	return r.sessionFromLineage(ctx, lineageID)
}

// RevokeUserSessions revokes every lineage belonging to the given user.
func (r *RefreshTokenService) RevokeUserSessions(ctx context.Context, userId string) error {
	sessions, err := r.ListSessionsByUser(ctx, userId)
	if err != nil {
		return err
	}
	for _, s := range sessions {
		if err := r.RevokeLineage(ctx, s.LineageId); err != nil {
			return err
		}
	}
	return nil
}

// CountSessions returns the number of active session lineages.
func (r *RefreshTokenService) CountSessions(ctx context.Context) (int64, error) {
	return r.Cache.ZCard(ctx, r.RefreshTokenPrefix+"sessions")
}
