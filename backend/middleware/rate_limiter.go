package middleware

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/Sephy314/chinwag/shared/response"
	"github.com/golang-jwt/jwt/v5"
	"github.com/labstack/echo/v5"
	"github.com/redis/go-redis/v9"
)

// RedisSlidingWindowStore implements RateLimiterStore using a sliding window
// counter backed by Redis sorted sets. Each request is recorded with its
// timestamp as the score, and expired entries are pruned on every check.
type RedisSlidingWindowStore struct {
	client *redis.Client
	limit  int
	window time.Duration
}

func NewRedisSlidingWindowStore(client *redis.Client, limit int, window time.Duration) *RedisSlidingWindowStore {
	return &RedisSlidingWindowStore{
		client: client,
		limit:  limit,
		window: window,
	}
}

func (s *RedisSlidingWindowStore) Allow(identifier string) (bool, error) {
	ctx := context.Background()
	key := fmt.Sprintf("rl:%s", identifier)
	now := time.Now()
	windowStart := now.Add(-s.window)

	pipe := s.client.Pipeline()
	pipe.ZRemRangeByScore(ctx, key, "0", strconv.FormatInt(windowStart.UnixNano(), 10))
	pipe.ZCard(ctx, key)
	pipe.ZAdd(ctx, key, redis.Z{
		Score:  float64(now.UnixNano()),
		Member: now.UnixNano(),
	})
	pipe.Expire(ctx, key, s.window+time.Second)

	results, err := pipe.Exec(ctx)
	if err != nil {
		return false, err
	}

	count := results[1].(*redis.IntCmd).Val()
	if count >= int64(s.limit) {
		return false, nil
	}

	return true, nil
}

// IPExtractor extracts the client IP from the request.
func IPExtractor(c *echo.Context) (string, error) {
	return c.RealIP(), nil
}

// JWTUserExtractor extracts the user ID (subject) from the JWT in context.
// Falls back to IP if no token is available.
func JWTUserExtractor(c *echo.Context) (string, error) {
	token, err := echo.ContextGet[*jwt.Token](c, "user")
	if err != nil {
		return c.RealIP(), nil
	}
	sub, err := token.Claims.GetSubject()
	if err != nil || sub == "" {
		return c.RealIP(), nil
	}
	return "user:" + sub, nil
}

// RateLimitDenyHandler returns a 429 JSON response with Retry-After header.
func RateLimitDenyHandler(c *echo.Context, _ string, _ error) error {
	c.Response().Header().Add("Retry-After", "60")
	return c.JSON(http.StatusTooManyRequests, response.Error("rate limit exceeded, try again later"))
}

// NewRateLimitMiddleware creates an Echo middleware that applies a per-key
// sliding window rate limit using the provided Redis store and extractor.
func NewRateLimitMiddleware(store *RedisSlidingWindowStore, extractor func(c *echo.Context) (string, error)) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) error {
			identifier, err := extractor(c)
			if err != nil {
				return next(c)
			}

			allowed, err := store.Allow(identifier)
			if err != nil {
				// On Redis failure, allow the request through
				return next(c)
			}
			if !allowed {
				return RateLimitDenyHandler(c, identifier, nil)
			}

			return next(c)
		}
	}
}
