package middleware

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

type RedisRateLimiterStore struct {
	client    *redis.Client
	rate      int
	burst     int
	expiresIn time.Duration
}

func NewRedisRateLimiterStore(client *redis.Client, rate, burst int, expiresIn time.Duration) *RedisRateLimiterStore {
	return &RedisRateLimiterStore{
		client:    client,
		rate:      rate,
		burst:     burst,
		expiresIn: expiresIn,
	}
}

func (s *RedisRateLimiterStore) Allow(identifier string) (bool, error) {
	ctx := context.Background()
	key := fmt.Sprintf("rate_limit:%s", identifier)

	pipe := s.client.Pipeline()
	incr := pipe.Incr(ctx, key)
	pipe.Expire(ctx, key, s.expiresIn)
	_, err := pipe.Exec(ctx)
	if err != nil {
		return false, err
	}

	count := incr.Val()
	if count > int64(s.burst) {
		return false, nil
	}

	return true, nil
}
