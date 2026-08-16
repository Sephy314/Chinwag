package middleware

import (
	"context"
	"fmt"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// testRedisAddr returns the Redis address for the integration tests, overridable
// via GATEWAY_TEST_REDIS_ADDR. Tests self-skip when no Redis is reachable
// (matches the repo's live-DB test convention, e.g. projectionRepo_test.go) so
// CI without Redis still passes. 127.0.0.1 (not "localhost") reaches WSL/Docker
// IPv4-only port mappings.
func testRedisAddr() string {
	if a := os.Getenv("GATEWAY_TEST_REDIS_ADDR"); a != "" {
		return a
	}
	return "127.0.0.1:6379"
}

// TestRedisLimiterIntegration exercises the real Lua-script limiter against a
// live Redis: allow/deny semantics, per-key TTL, and atomicity under
// concurrency. Skipped when Redis is not available.
func TestRedisLimiterIntegration(t *testing.T) {
	rl := NewRedisLimiter(testRedisAddr(), "", 20, 60, 120*time.Second, 200*time.Millisecond)
	defer rl.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := rl.Ping(ctx); err != nil {
		t.Skipf("redis not reachable at %s (skipping integration test): %v", testRedisAddr(), err)
	}

	t.Run("allow then deny after burst", func(t *testing.T) {
		rl3 := NewRedisLimiter(testRedisAddr(), "", 0.0001, 3, 120*time.Second, 200*time.Millisecond)
		defer rl3.Close()
		id := fmt.Sprintf("test-burst-%d", time.Now().UnixNano())

		for i := 0; i < 3; i++ {
			ok, err := rl3.Allow(context.Background(), id)
			if err != nil {
				t.Fatalf("allow %d: %v", i+1, err)
			}
			if !ok {
				t.Fatalf("allow %d: expected true (burst %d)", i+1, 3)
			}
		}
		ok, err := rl3.Allow(context.Background(), id)
		if err != nil {
			t.Fatal(err)
		}
		if ok {
			t.Fatal("expected deny after burst consumed")
		}
	})

	t.Run("key has a TTL", func(t *testing.T) {
		rl2 := NewRedisLimiter(testRedisAddr(), "", 0.0001, 3, 120*time.Second, 200*time.Millisecond)
		defer rl2.Close()
		id := fmt.Sprintf("test-ttl-%d", time.Now().UnixNano())
		if _, err := rl2.Allow(context.Background(), id); err != nil {
			t.Fatal(err)
		}
		ttl, err := rl2.client.TTL(context.Background(), redisRateLimitKeyPrefix+id).Result()
		if err != nil {
			t.Fatal(err)
		}
		if ttl <= 0 || ttl > 120*time.Second {
			t.Fatalf("unexpected key TTL %v (want 0 < ttl <= 120s)", ttl)
		}
	})

	t.Run("atomic under concurrency", func(t *testing.T) {
		const burst = 10
		rlA := NewRedisLimiter(testRedisAddr(), "", 0.0001, burst, 120*time.Second, 200*time.Millisecond)
		defer rlA.Close()
		id := fmt.Sprintf("test-atomic-%d", time.Now().UnixNano())

		var wg sync.WaitGroup
		var allowed atomic.Int32
		for i := 0; i < burst*3; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				ok, err := rlA.Allow(context.Background(), id)
				if err == nil && ok {
					allowed.Add(1)
				}
			}()
		}
		wg.Wait()
		if got := allowed.Load(); got != burst {
			t.Fatalf("expected exactly %d allowed under concurrency, got %d", burst, got)
		}
	})

	t.Run("context cancellation returns error", func(t *testing.T) {
		rlC := NewRedisLimiter(testRedisAddr(), "", 20, 60, 120*time.Second, 200*time.Millisecond)
		defer rlC.Close()
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		if _, err := rlC.Allow(ctx, "test-cancel"); err == nil {
			t.Fatal("expected error when context is cancelled")
		}
	})
}
