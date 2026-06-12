package cache

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

type Cache interface {
	Get(ctx context.Context, key string) (string, error)
	Set(ctx context.Context, key string, value any, ttl time.Duration) error
	Delete(ctx context.Context, key string) error
	TTL(ctx context.Context, key string) (time.Duration, error)
}

type RedisCache struct {
	client *redis.Client
}

type MockedCache struct {
	client *redis.Client
}

func NewRedisCache(rds *redis.Client) *RedisCache {
	return &RedisCache{
		client: rds,
	}
}

func (rc *RedisCache) Get(ctx context.Context, key string) (string, error) {
	return rc.client.Get(ctx, key).Result()
}

func (rc *RedisCache) Set(ctx context.Context, key string, value any, ttl time.Duration) error {
	return rc.client.Set(ctx, key, value, ttl).Err()
}

func (rc *RedisCache) Delete(ctx context.Context, key string) error {
	return rc.client.Del(ctx, key).Err()
}

func (rc *RedisCache) TTL(ctx context.Context, key string) (time.Duration, error) {
	return rc.client.TTL(ctx, key).Result()
}

func (mocked *MockedCache) Get(ctx context.Context, key string) (string, error) {
	return mocked.client.Get(ctx, key).Result()
}

func (mocked *MockedCache) Set(ctx context.Context, key string, value any, ttl time.Duration) error {
	return mocked.client.Set(ctx, key, value, ttl).Err()
}

func (mocked *MockedCache) Delete(ctx context.Context, key string) error {
	return mocked.client.Del(ctx, key).Err()
}

func (mocked *MockedCache) TTL(ctx context.Context, key string) (time.Duration, error) {
	return mocked.client.TTL(ctx, key).Result()
}
