package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"strconv"
	"time"

	"github.com/Sephy314/chinwag/backend/services/auth/shared/cache"
	"github.com/Sephy314/chinwag/backend/services/auth/shared/errs"
	"github.com/Sephy314/chinwag/backend/services/auth/structs"
	"github.com/google/uuid"
)

const consumeRefreshTokenScript = `
if redis.call('EXISTS', KEYS[1]) == 0 then
  return {'NOT_FOUND', '', '', ''}
end
local rec = redis.call('HGETALL', KEYS[1])
local t = {}
for i = 1, #rec, 2 do
  t[rec[i]] = rec[i + 1]
end
if t['revoked'] == '1' then
  return {'REVOKED', '', '', ''}
end
if t['used'] == '1' then
  return {'USED', t['user_id'], t['lineage_id'], t['parent_hash']}
end
redis.call('HSET', KEYS[1], 'used', '1')
return {'OK', t['user_id'], t['lineage_id'], t['parent_hash']}
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
		"used":        "0",
		"revoked":     "0",
		"created_at":  strconv.FormatInt(time.Now().Unix(), 10),
	}

	if err := r.Cache.HSet(ctx, r.RefreshTokenPrefix+token.RefreshToken, fields, r.RefreshTokenTTL); err != nil {
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
	if !ok || len(arr) < 4 {
		return nil, errs.ErrCacheNotFound
	}

	record := &structs.RefreshTokenRecord{
		UserID:     asString(arr[1]),
		LineageID:  asString(arr[2]),
		ParentHash: asString(arr[3]),
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

	return r.Cache.Delete(ctx, lineageKey)
}

func (r *RefreshTokenService) toRecord(fields map[string]string) *structs.RefreshTokenRecord {
	createdAt, _ := strconv.ParseInt(fields["created_at"], 10, 64)
	return &structs.RefreshTokenRecord{
		UserID:     fields["user_id"],
		LineageID:  fields["lineage_id"],
		ParentHash: fields["parent_hash"],
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
