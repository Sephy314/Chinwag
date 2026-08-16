package middleware

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	// redisRateLimitKeyPrefix namespaces the per-IP token buckets in Redis.
	redisRateLimitKeyPrefix = "gateway:ratelimit:"
)

// allowTokensScript is an atomic token-bucket consume in Redis. KEYS[1] is the
// per-IP bucket key. ARGV: [rate, burst, now_ms, ttl_s].
//
// Returns 1 if a token was consumed (request allowed), 0 otherwise. The bucket
// refills at `rate` tokens/sec up to `burst`, and every call renews the key TTL
// so idle IPs' keys expire and never accumulate. It is a single EVALSHA round
// trip, so it is atomic and cheap.
var allowTokensScript = redis.NewScript(`
local v = redis.call('HMGET', KEYS[1], 'tokens', 'ts')
local tokens = tonumber(v[1])
local ts = tonumber(v[2])
local now = tonumber(ARGV[3])
local burst = tonumber(ARGV[2])
local rate = tonumber(ARGV[1])
if tokens == nil then tokens = burst end
if ts == nil then ts = now end
local refill = ((now - ts) / 1000.0) * rate
tokens = math.min(burst, tokens + refill)
local allowed = 0
if tokens >= 1 then
  tokens = tokens - 1
  allowed = 1
end
redis.call('HSET', KEYS[1], 'tokens', tokens, 'ts', now)
redis.call('EXPIRE', KEYS[1], ARGV[4])
return allowed
`)

// Limiter is the authoritative distributed rate limiter used by the rate-limit
// middleware. RedisLimiter implements it; tests inject a fake.
type Limiter interface {
	// Allow consumes one token for identifier, returning true when the request
	// is within the limit. A non-nil error means the limiter could not be
	// consulted (e.g. Redis down) — the middleware then falls back to L1.
	Allow(ctx context.Context, identifier string) (bool, error)
}

// RedisLimiter is a token-bucket limiter backed by Redis. Redis is the
// authoritative distributed rate-limit state, so multiple gateway replicas
// share one per-IP budget. Every request is a single atomic EVALSHA; the key
// is created lazily (no preload) and expires after ttl of inactivity.
type RedisLimiter struct {
	client *redis.Client
	rate   float64
	burst  int
	ttl    time.Duration
}

// NewRedisLimiter returns a Redis-backed limiter. The client gets explicit
// dial/read/write timeouts so a slow or unreachable Redis cannot stall a
// request indefinitely — the middleware treats such errors as an L1 fallback
// trigger (fail-open, never fail-closed).
func NewRedisLimiter(addr, password string, rate float64, burst int, ttl, timeout time.Duration) *RedisLimiter {
	client := redis.NewClient(&redis.Options{
		Addr:         addr,
		Password:     password,
		DialTimeout:  timeout,
		ReadTimeout:  timeout,
		WriteTimeout: timeout,
	})
	return &RedisLimiter{
		client: client,
		rate:   rate,
		burst:  burst,
		ttl:    ttl,
	}
}

// Ping reports whether Redis is reachable (used by tests / startup checks).
func (l *RedisLimiter) Ping(ctx context.Context) error {
	return l.client.Ping(ctx).Err()
}

// Allow consumes one token for identifier via the atomic Lua script.
func (l *RedisLimiter) Allow(ctx context.Context, identifier string) (bool, error) {
	key := redisRateLimitKeyPrefix + identifier
	res, err := allowTokensScript.Run(ctx, l.client, []string{key},
		l.rate, l.burst, time.Now().UnixMilli(), int64(l.ttl.Seconds()),
	).Int()
	if err != nil {
		return false, err
	}
	return res == 1, nil
}

// Close releases the Redis connection pool.
func (l *RedisLimiter) Close() error {
	return l.client.Close()
}
