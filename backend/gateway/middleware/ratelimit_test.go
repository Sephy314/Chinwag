package middleware

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/labstack/echo/v5"
	"github.com/prometheus/client_golang/prometheus/testutil"
)

// --- helpers ---------------------------------------------------------------

func doGet(e *echo.Echo, path string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, path, nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	return rec
}

// doGetXFF sends a request as if it came through Traefik (private RemoteAddr,
// trusted by ExtractIPFromXFFHeader) so the XFF client IP is the limiter key.
func doGetXFF(e *echo.Echo, path, ip string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, path, nil)
	req.Header.Set("X-Forwarded-For", ip)
	req.RemoteAddr = "10.42.0.1:1234"
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	return rec
}

// newRateLimitedEcho builds an Echo with the limiter registered and the XFF
// IP extractor set (as main.go does) so per-client-IP keying works via XFF.
func newRateLimitedEcho(cfg RateLimitConfig) *echo.Echo {
	e := echo.New()
	e.IPExtractor = echo.ExtractIPFromXFFHeader()
	e.Use(RateLimit(cfg))
	return e
}

// l1OnlyCfg returns a config with no Redis (memory backend).
func l1OnlyCfg(rate float64, burst int) RateLimitConfig {
	return RateLimitConfig{
		Enabled:      true,
		Rate:         rate,
		Burst:        burst,
		Redis:        nil,
		RedisTimeout: time.Second,
		L1ExpiresIn:  time.Minute,
	}
}

// redisCfg returns a config whose Redis layer is the given limiter and whose
// L1 is large enough to never be the deciding layer (isolates Redis tests).
func redisCfg(r Limiter, timeout time.Duration) RateLimitConfig {
	return RateLimitConfig{
		Enabled:      true,
		Rate:         1e9, // L1 huge → Redis is authoritative in these tests
		Burst:        1 << 30,
		Redis:        r,
		RedisTimeout: timeout,
		L1ExpiresIn:  time.Minute,
	}
}

// --- fake Redis limiter ----------------------------------------------------

// fakeRedis is an in-memory token bucket that mirrors the Lua script. Setting
// err simulates a Redis outage; clearing it simulates recovery.
type fakeRedis struct {
	mu     sync.Mutex
	rate   float64
	burst  int
	tokens map[string]float64
	ts     map[string]int64
	err    error
}

func newFakeRedis(rate float64, burst int) *fakeRedis {
	return &fakeRedis{rate: rate, burst: burst, tokens: map[string]float64{}, ts: map[string]int64{}}
}

func (f *fakeRedis) Allow(_ context.Context, id string) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.err != nil {
		return false, f.err
	}
	now := time.Now().UnixMilli()
	tok, ok := f.tokens[id]
	if !ok {
		tok = float64(f.burst)
	}
	ts, _ := f.ts[id]
	tok += float64(now-ts) / 1000.0 * f.rate
	if tok > float64(f.burst) {
		tok = float64(f.burst)
	}
	allowed := false
	if tok >= 1 {
		tok--
		allowed = true
	}
	f.tokens[id] = tok
	f.ts[id] = now
	return allowed, nil
}

// blockingRedis blocks until the context expires (simulates a Redis timeout).
type blockingRedis struct{}

func (blockingRedis) Allow(ctx context.Context, _ string) (bool, error) {
	<-ctx.Done()
	return false, ctx.Err()
}

// --- memory backend (no Redis) — existing PR #62 behavior ------------------

func TestRateLimitDeniesBeyondBurst(t *testing.T) {
	e := echo.New()
	e.Use(RateLimit(l1OnlyCfg(100, 3)))
	e.GET("/", func(c *echo.Context) error { return c.String(http.StatusOK, "ok") })

	for i := 0; i < 3; i++ {
		if rec := doGet(e, "/"); rec.Code != http.StatusOK {
			t.Fatalf("request %d: got %d, want 200", i+1, rec.Code)
		}
	}
	if rec := doGet(e, "/"); rec.Code != http.StatusTooManyRequests {
		t.Fatalf("4th request: got %d, want 429", rec.Code)
	}
}

func TestRateLimitSkipsHealthAndMetrics(t *testing.T) {
	e := echo.New()
	e.Use(RateLimit(l1OnlyCfg(100, 1)))
	e.GET("/", func(c *echo.Context) error { return c.String(http.StatusOK, "ok") })
	e.GET("/health", func(c *echo.Context) error { return c.String(http.StatusOK, "ok") })
	e.GET("/metrics", func(c *echo.Context) error { return c.String(http.StatusOK, "ok") })

	for i := 0; i < 5; i++ {
		if rec := doGet(e, "/health"); rec.Code != http.StatusOK {
			t.Fatalf("health #%d: got %d, want 200 (skipped)", i+1, rec.Code)
		}
		if rec := doGet(e, "/metrics"); rec.Code != http.StatusOK {
			t.Fatalf("metrics #%d: got %d, want 200 (skipped)", i+1, rec.Code)
		}
	}
	if rec := doGet(e, "/"); rec.Code != http.StatusOK {
		t.Fatalf("first /: got %d, want 200", rec.Code)
	}
	if rec := doGet(e, "/"); rec.Code != http.StatusTooManyRequests {
		t.Fatalf("second /: got %d, want 429", rec.Code)
	}
}

func TestRateLimitDisabledIsNoop(t *testing.T) {
	e := echo.New()
	e.Use(RateLimit(RateLimitConfig{Enabled: false, Rate: 0, Burst: 0}))
	e.GET("/", func(c *echo.Context) error { return c.String(http.StatusOK, "ok") })

	for i := 0; i < 25; i++ {
		if rec := doGet(e, "/"); rec.Code != http.StatusOK {
			t.Fatalf("request %d: got %d, want 200", i+1, rec.Code)
		}
	}
}

// TestRateLimitFullChain mirrors main.go: XFF extractor, the Use chain, and
// RateLimit as a Pre middleware registered BEFORE a short-circuiting Pre proxy
// (like setupRoutes in proxy.go). Guards the proxy-bypass regression.
func TestRateLimitFullChain(t *testing.T) {
	e := echo.New()
	e.IPExtractor = echo.ExtractIPFromXFFHeader()
	e.Use(RequestID())
	e.Use(AccessLogger())
	e.Use(MetricsMiddleware())
	e.Pre(RateLimit(l1OnlyCfg(0.001, 2)))
	e.Pre(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) error {
			if c.Request().URL.Path == "/chat" {
				return c.String(http.StatusServiceUnavailable, "proxy-unavailable")
			}
			return next(c)
		}
	})
	e.GET("/chat", func(c *echo.Context) error { return c.String(http.StatusOK, "ok") })

	for i := 0; i < 2; i++ {
		if got := doGetXFF(e, "/chat", "203.0.113.5").Code; got != http.StatusServiceUnavailable {
			t.Fatalf("request %d (same IP): got %d, want 503 from proxy", i+1, got)
		}
	}
	if got := doGetXFF(e, "/chat", "203.0.113.5").Code; got != http.StatusTooManyRequests {
		t.Fatalf("3rd request (same IP): got %d, want 429 from limiter", got)
	}
	if got := doGetXFF(e, "/chat", "198.51.100.9").Code; got != http.StatusServiceUnavailable {
		t.Fatalf("different IP: got %d, want 503 (own budget)", got)
	}
}

// --- Redis authoritative (normal mode) -------------------------------------

func TestRateLimitRedisSharesBudgetAnd429s(t *testing.T) {
	redis := newFakeRedis(0.001, 2) // burst 2, no refill during test
	e := newRateLimitedEcho(redisCfg(redis, time.Second))
	e.GET("/", func(c *echo.Context) error { return c.String(http.StatusOK, "ok") })

	for i := 0; i < 2; i++ {
		if got := doGetXFF(e, "/", "203.0.113.5").Code; got != http.StatusOK {
			t.Fatalf("request %d (same IP): got %d, want 200", i+1, got)
		}
	}
	if got := doGetXFF(e, "/", "203.0.113.5").Code; got != http.StatusTooManyRequests {
		t.Fatalf("3rd (same IP): got %d, want 429 (Redis authoritative)", got)
	}
}

func TestRateLimitRedisDifferentIPOwnBucket(t *testing.T) {
	redis := newFakeRedis(0.001, 2)
	e := newRateLimitedEcho(redisCfg(redis, time.Second))
	e.GET("/", func(c *echo.Context) error { return c.String(http.StatusOK, "ok") })

	doGetXFF(e, "/", "203.0.113.5")
	doGetXFF(e, "/", "203.0.113.5")
	if got := doGetXFF(e, "/", "203.0.113.5").Code; got != http.StatusTooManyRequests {
		t.Fatalf("IP A exhausted: got %d, want 429", got)
	}
	// A different IP still has its own Redis bucket.
	if got := doGetXFF(e, "/", "198.51.100.9").Code; got != http.StatusOK {
		t.Fatalf("different IP: got %d, want 200 (own bucket)", got)
	}
}

func TestRateLimitRedisConcurrentAtomicity(t *testing.T) {
	const burst = 5
	redis := newFakeRedis(0.0000001, burst)
	e := newRateLimitedEcho(redisCfg(redis, time.Second))
	e.GET("/", func(c *echo.Context) error { return c.String(http.StatusOK, "ok") })

	var wg sync.WaitGroup
	var allowed atomic.Int32
	for i := 0; i < burst*3; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if rec := doGetXFF(e, "/", "203.0.113.5"); rec.Code == http.StatusOK {
				allowed.Add(1)
			}
		}()
	}
	wg.Wait()
	if got := allowed.Load(); got != burst {
		t.Fatalf("expected exactly %d allowed under concurrency, got %d", burst, got)
	}
}

// --- Redis failure → L1 fallback (fail-open) -------------------------------

func TestRateLimitRedisErrorFallsBackToL1(t *testing.T) {
	redis := newFakeRedis(0.001, 2)
	redis.err = errors.New("connection refused")

	// L1 burst 2 so L1 actually decides during the fallback.
	cfg := l1OnlyCfg(0.001, 2)
	cfg.Redis = redis
	cfg.RedisTimeout = time.Second

	e := newRateLimitedEcho(cfg)
	e.GET("/", func(c *echo.Context) error { return c.String(http.StatusOK, "ok") })

	before := testutil.ToFloat64(rateLimitL1Fallback)
	// Requests are NOT failed closed: L1 allows 2, then L1 denies.
	for i := 0; i < 2; i++ {
		if got := doGetXFF(e, "/", "203.0.113.5").Code; got != http.StatusOK {
			t.Fatalf("request %d (Redis down): got %d, want 200 (L1 fallback)", i+1, got)
		}
	}
	if got := doGetXFF(e, "/", "203.0.113.5").Code; got != http.StatusTooManyRequests {
		t.Fatalf("3rd (Redis down): got %d, want 429 (L1 still limits)", got)
	}
	after := testutil.ToFloat64(rateLimitL1Fallback)
	if after <= before {
		t.Error("rate_limit_l1_fallback_total was not incremented")
	}
}

func TestRateLimitRedisTimeoutFallsBackToL1(t *testing.T) {
	e := newRateLimitedEcho(redisCfg(blockingRedis{}, 50*time.Millisecond))
	e.GET("/", func(c *echo.Context) error { return c.String(http.StatusOK, "ok") })

	// Redis blocks past the 50ms deadline → error → L1 (huge) allows.
	if got := doGetXFF(e, "/", "203.0.113.5").Code; got != http.StatusOK {
		t.Fatalf("Redis timeout: got %d, want 200 (L1 fallback)", got)
	}
}

// --- Redis recovery --------------------------------------------------------

// TestRateLimitRedisRecovery exercises:
//
//	Redis healthy → Redis decides → Redis down → L1 fallback → Redis restored
//	→ Redis authoritative again.
//
// It also proves L1 state is NOT written back to Redis: IP A's Redis bucket is
// consumed in phase 1, untouched during the L1-only phase 2, and still empty
// when Redis returns (no write-back / no reconciliation) — while a fresh IP B
// gets a full Redis bucket and is limited by Redis again.
func TestRateLimitRedisRecovery(t *testing.T) {
	redis := newFakeRedis(0.001, 2) // Redis burst 2, no refill
	cfg := redisCfg(redis, time.Second)
	// Keep L1 huge so it never interferes — Redis (or the fallback) decides.
	e := newRateLimitedEcho(cfg)
	e.GET("/", func(c *echo.Context) error { return c.String(http.StatusOK, "ok") })

	// Phase 1: Redis healthy → authoritative (burst 2).
	if got := doGetXFF(e, "/", "203.0.113.5").Code; got != http.StatusOK {
		t.Fatalf("phase1 req1: got %d, want 200", got)
	}
	if got := doGetXFF(e, "/", "203.0.113.5").Code; got != http.StatusOK {
		t.Fatalf("phase1 req2: got %d, want 200", got)
	}
	if got := doGetXFF(e, "/", "203.0.113.5").Code; got != http.StatusTooManyRequests {
		t.Fatalf("phase1 req3: got %d, want 429 (Redis exhausted)", got)
	}

	// Phase 2: Redis down → L1-only (L1 huge → allowed, fail-open). Redis
	// bucket for A stays empty — L1 writes nothing back.
	redis.err = errors.New("connection refused")
	if got := doGetXFF(e, "/", "203.0.113.5").Code; got != http.StatusOK {
		t.Fatalf("phase2 (Redis down): got %d, want 200 (L1 fallback)", got)
	}

	// Phase 3: Redis restored → authoritative again.
	redis.err = nil
	// A's Redis bucket is still empty (untouched during L1-only phase) → denied.
	if got := doGetXFF(e, "/", "203.0.113.5").Code; got != http.StatusTooManyRequests {
		t.Fatalf("phase3 A (no write-back): got %d, want 429 (Redis bucket still empty)", got)
	}
	// A fresh IP has a full Redis bucket → Redis allows, then denies.
	for i := 0; i < 2; i++ {
		if got := doGetXFF(e, "/", "198.51.100.9").Code; got != http.StatusOK {
			t.Fatalf("phase3 B req%d: got %d, want 200 (Redis authoritative)", i+1, got)
		}
	}
	if got := doGetXFF(e, "/", "198.51.100.9").Code; got != http.StatusTooManyRequests {
		t.Fatalf("phase3 B req3: got %d, want 429 (Redis authoritative)", got)
	}
}
