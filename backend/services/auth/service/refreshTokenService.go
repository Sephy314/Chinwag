package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"sort"
	"strconv"
	"strings"
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

	fields := map[string]string{
		"user_id":     token.Subject,
		"lineage_id":  lineageID,
		"parent_hash": token.ParentHash,
		"jkt":         token.Jkt,
		"used":        "0",
		"revoked":     "0",
		"created_at":  strconv.FormatInt(time.Now().Unix(), 10),
	}

	if err := r.Cache.HSet(ctx, r.RefreshTokenPrefix+token.RefreshToken, fields, r.RefreshTokenTTL); err != nil {
		return err
	}

	// Maintain the user -> lineage index so admin/session listing can find a
	// user's sessions without scanning every token key.
	if err := r.Cache.SAdd(ctx, r.RefreshTokenPrefix+"user:"+token.Subject, r.RefreshTokenTTL, lineageID); err != nil {
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

	// Remove the lineage from its owner's user -> lineage index.
	if len(members) > 0 {
		if rec, rerr := r.getRecord(ctx, members[0]); rerr == nil && rec.UserID != "" {
			_ = r.Cache.SRem(ctx, r.RefreshTokenPrefix+"user:"+rec.UserID, lineageID)
		}
	}

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

// ListSessionsByUser returns all lineages (sessions) belonging to a user.
func (r *RefreshTokenService) ListSessionsByUser(ctx context.Context, userId string) ([]structs.AdminSession, error) {
	lineages, err := r.Cache.SMembers(ctx, r.RefreshTokenPrefix+"user:"+userId)
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
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt > out[j].CreatedAt })
	return out, nil
}

// ListAllSessions returns every active session (lineage) across all users.
func (r *RefreshTokenService) ListAllSessions(ctx context.Context) ([]structs.AdminSession, error) {
	seen := map[string]bool{}
	out := []structs.AdminSession{}
	var cursor uint64
	for {
		keys, next, err := r.Cache.Scan(ctx, cursor, r.RefreshTokenPrefix+"*", 100)
		if err != nil {
			return nil, err
		}
		for _, k := range keys {
			if strings.HasPrefix(k, r.RefreshTokenPrefix+"user:") || strings.HasPrefix(k, r.RefreshTokenPrefix+"lineage:") {
				continue
			}
			rec, err := r.getRecord(ctx, strings.TrimPrefix(k, r.RefreshTokenPrefix))
			if err != nil || rec.LineageID == "" || seen[rec.LineageID] {
				continue
			}
			seen[rec.LineageID] = true
			if s, err := r.sessionFromLineage(ctx, rec.LineageID); err == nil && s != nil {
				out = append(out, *s)
			}
		}
		cursor = next
		if next == 0 {
			break
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt > out[j].CreatedAt })
	return out, nil
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
	var count int64
	var cursor uint64
	for {
		keys, next, err := r.Cache.Scan(ctx, cursor, r.RefreshTokenPrefix+"lineage:*", 100)
		if err != nil {
			return 0, err
		}
		count += int64(len(keys))
		cursor = next
		if next == 0 {
			break
		}
	}
	return count, nil
}
