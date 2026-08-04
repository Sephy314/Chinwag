package cache

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

type Cache interface {
	Get(ctx context.Context, key string) (string, error)
	Set(ctx context.Context, key string, value any, ttl time.Duration) error
	SetNX(ctx context.Context, key string, value any, ttl time.Duration) (bool, error)
	Delete(ctx context.Context, key string) error
	TTL(ctx context.Context, key string) (time.Duration, error)
	HSet(ctx context.Context, key string, fields map[string]string, ttl time.Duration) error
	HGetAll(ctx context.Context, key string) (map[string]string, error)
	SAdd(ctx context.Context, key string, ttl time.Duration, members ...string) error
	SMembers(ctx context.Context, key string) ([]string, error)
	Eval(ctx context.Context, script string, keys []string, args ...any) (any, error)
	AcquireLock(ctx context.Context, key string, token string, ttl time.Duration) (bool, error)
	ReleaseLock(ctx context.Context, key string, token string) error
}

type RedisCache struct {
	client *redis.Client
}

func NewRedisCache(rds *redis.Client) *RedisCache {
	return &RedisCache{client: rds}
}

func (rc *RedisCache) Get(ctx context.Context, key string) (string, error) {
	return rc.client.Get(ctx, key).Result()
}

func (rc *RedisCache) Set(ctx context.Context, key string, value any, ttl time.Duration) error {
	return rc.client.Set(ctx, key, value, ttl).Err()
}

func (rc *RedisCache) SetNX(ctx context.Context, key string, value any, ttl time.Duration) (bool, error) {
	return rc.client.SetNX(ctx, key, value, ttl).Result()
}

func (rc *RedisCache) Delete(ctx context.Context, key string) error {
	return rc.client.Del(ctx, key).Err()
}

func (rc *RedisCache) TTL(ctx context.Context, key string) (time.Duration, error) {
	return rc.client.TTL(ctx, key).Result()
}

func (rc *RedisCache) HSet(ctx context.Context, key string, fields map[string]string, ttl time.Duration) error {
	if err := rc.client.HSet(ctx, key, fields).Err(); err != nil {
		return err
	}
	if ttl > 0 {
		return rc.client.Expire(ctx, key, ttl).Err()
	}
	return nil
}

func (rc *RedisCache) HGetAll(ctx context.Context, key string) (map[string]string, error) {
	return rc.client.HGetAll(ctx, key).Result()
}

func (rc *RedisCache) SAdd(ctx context.Context, key string, ttl time.Duration, members ...string) error {
	if err := rc.client.SAdd(ctx, key, members).Err(); err != nil {
		return err
	}
	if ttl > 0 {
		return rc.client.Expire(ctx, key, ttl).Err()
	}
	return nil
}

func (rc *RedisCache) SMembers(ctx context.Context, key string) ([]string, error) {
	return rc.client.SMembers(ctx, key).Result()
}

func (rc *RedisCache) Eval(ctx context.Context, script string, keys []string, args ...any) (any, error) {
	return rc.client.Eval(ctx, script, keys, args...).Result()
}

func (rc *RedisCache) AcquireLock(ctx context.Context, key string, token string, ttl time.Duration) (bool, error) {
	res, err := rc.client.SetNX(ctx, key, token, ttl).Result()
	if err != nil {
		return false, err
	}
	return res, nil
}

const releaseLockScript = `
if redis.call('GET', KEYS[1]) == ARGV[1] then
  return redis.call('DEL', KEYS[1])
end
return 0
`

func (rc *RedisCache) ReleaseLock(ctx context.Context, key string, token string) error {
	return rc.client.Eval(ctx, releaseLockScript, []string{key}, token).Err()
}
