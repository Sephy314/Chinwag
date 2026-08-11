package service

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"strconv"
	"time"

	"github.com/Sephy314/chinwag/backend/services/auth/shared/cache"
	"github.com/Sephy314/chinwag/backend/services/auth/shared/errs"
	"github.com/Sephy314/chinwag/backend/services/auth/structs"
	"github.com/google/uuid"
)

const consumeRefreshTokenScript = `
if redis.call('EXISTS', KEYS[1]) == 0 then
  return {'NOT_FOUND', '', '', '', ''}
end
local rec = redis.call('HGETALL', KEYS[1])
local t = {}
for i = 1, #rec, 2 do
  t[rec[i]] = rec[i + 1]
end
if t['revoked'] == '1' then
  return {'REVOKED', '', '', '', ''}
end
if t['used'] == '1' then
  return {'USED', t['user_id'], t['lineage_id'], t['parent_hash'], t['jkt']}
end
redis.call('HSET', KEYS[1], 'used', '1')
return {'OK', t['user_id'], t['lineage_id'], t['parent_hash'], t['jkt']}
`

type RefreshTokenServiceInterface interface {
	GetRefreshToken(ctx context.Context, refreshToken string) (*structs.RefreshTokenRecord, error)
	InsertRefreshToken(ctx context.Context, token structs.RefreshToken) error
	ConsumeRefreshToken(ctx context.Context, refreshToken string) (*structs.RefreshTokenRecord, error)
	RevokeLineage(ctx context.Context, lineageID string) error
}

type RefreshTokenService struct {
	Cache              cache.Cache
	RefreshTokenPrefix string
	RefreshTokenTTL    time.Duration
}

func (r *RefreshTokenService) GetRefreshToken(ctx context.Context, refreshToken string) (*structs.RefreshTokenRecord, error) {
	fields, err := r.Cache.HGetAll(ctx, r.RefreshTokenPrefix+refreshToken)
	if err != nil {
		return nil, err
	}
	if len(fields) == 0 {
		return nil, errs.ErrCacheNotFound
	}
	return r.toRecord(fields), nil
}

func (r *RefreshTokenService) InsertRefreshToken(ctx context.Context, token structs.RefreshToken) error {
	lineageID := token.LineageID
	if lineageID == "" {
		lineageID = uuid.Must(uuid.NewV7()).String()
	}

	now := time.Now().Unix()
	fields := map[string]string{
		"user_id":     token.Subject,
		"lineage_id":  lineageID,
		"parent_hash": token.ParentHash,
		"jkt":         token.Jkt,
		"used":        "0",
		"revoked":     "0",
		"created_at":  strconv.FormatInt(now, 10),
	}

	if err := r.Cache.HSet(ctx, r.RefreshTokenPrefix+token.RefreshToken, fields, r.RefreshTokenTTL); err != nil {
		return err
	}

	// Maintain the per-user and global session indexes as Sorted Sets so
	// sessions can be listed with ordered, cursor-based range queries instead
	// of scanning the whole Redis keyspace.
	if err := r.Cache.ZAdd(ctx, r.RefreshTokenPrefix+"user:"+token.Subject, now, lineageID); err != nil {
		return err
	}
	if err := r.Cache.ZAdd(ctx, r.RefreshTokenPrefix+"sessions", now, lineageID); err != nil {
		return err
	}

	return r.Cache.SAdd(ctx, r.RefreshTokenPrefix+"lineage:"+lineageID, r.RefreshTokenTTL, token.RefreshToken)
}

func (r *RefreshTokenService) ConsumeRefreshToken(ctx context.Context, refreshToken string) (*structs.RefreshTokenRecord, error) {
	result, err := r.Cache.Eval(ctx, consumeRefreshTokenScript, []string{r.RefreshTokenPrefix + refreshToken})
	if err != nil {
		return nil, err
	}

	arr, ok := result.([]interface{})
	if !ok || len(arr) < 5 {
		return nil, errs.ErrCacheNotFound
	}

	record := &structs.RefreshTokenRecord{
		UserID:     asString(arr[1]),
		LineageID:  asString(arr[2]),
		ParentHash: asString(arr[3]),
		Jkt:        asString(arr[4]),
	}

	switch asString(arr[0]) {
	case "NOT_FOUND":
		return nil, errs.ErrCacheNotFound
	case "REVOKED":
		record.Revoked = true
		return record, errs.ErrRefreshTokenRevoked
	case "USED":
		record.Used = true
		return record, errs.ErrRefreshTokenReused
	case "OK":
		return record, nil
	default:
		return nil, errs.ErrCacheNotFound
	}
}

func (r *RefreshTokenService) RevokeLineage(ctx context.Context, lineageID string) error {
	if lineageID == "" {
		return nil
	}

	lineageKey := r.RefreshTokenPrefix + "lineage:" + lineageID
	members, err := r.Cache.SMembers(ctx, lineageKey)
	if err != nil {
		return err
	}

	for _, token := range members {
		if err := r.Cache.HSet(ctx, r.RefreshTokenPrefix+token, map[string]string{"revoked": "1"}, 0); err != nil {
			return err
		}
	}

	// Remove the lineage from the per-user and global session indexes.
	if len(members) > 0 {
		if rec, rerr := r.getRecord(ctx, members[0]); rerr == nil && rec.UserID != "" {
			_ = r.Cache.ZRem(ctx, r.RefreshTokenPrefix+"user:"+rec.UserID, lineageID)
		}
	}
	_ = r.Cache.ZRem(ctx, r.RefreshTokenPrefix+"sessions", lineageID)

	return r.Cache.Delete(ctx, lineageKey)
}

func (r *RefreshTokenService) toRecord(fields map[string]string) *structs.RefreshTokenRecord {
	createdAt, _ := strconv.ParseInt(fields["created_at"], 10, 64)
	return &structs.RefreshTokenRecord{
		UserID:     fields["user_id"],
		LineageID:  fields["lineage_id"],
		ParentHash: fields["parent_hash"],
		Jkt:        fields["jkt"],
		Used:       fields["used"] == "1",
		Revoked:    fields["revoked"] == "1",
		CreatedAt:  createdAt,
	}
}

func HashRefreshToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func asString(v any) string {
	s, _ := v.(string)
	return s
}

func NewRefreshTokenService(cache cache.Cache, prefix string, ttl time.Duration) *RefreshTokenService {
	return &RefreshTokenService{
		Cache:              cache,
		RefreshTokenPrefix: prefix,
		RefreshTokenTTL:    ttl,
	}
}

// --- Admin session introspection ---

func (r *RefreshTokenService) getRecord(ctx context.Context, token string) (*structs.RefreshTokenRecord, error) {
	fields, err := r.Cache.HGetAll(ctx, r.RefreshTokenPrefix+token)
	if err != nil {
		return nil, err
	}
	if len(fields) == 0 {
		return nil, errs.ErrCacheNotFound
	}
	return r.toRecord(fields), nil
}

// sessionFromLineage builds a safe AdminSession for a lineage (one login). The
// raw refresh token value is never included.
func (r *RefreshTokenService) sessionFromLineage(ctx context.Context, lineageID string) (*structs.AdminSession, error) {
	members, err := r.Cache.SMembers(ctx, r.RefreshTokenPrefix+"lineage:"+lineageID)
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
