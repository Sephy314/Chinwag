package middleware

import (
	"sync"
	"time"
)

type cbState int

const (
	cbClosed cbState = iota
	cbOpen
	cbHalfOpen
)

// circuitBreaker is a minimal trip/cooldown/probe breaker around the Redis
// rate-limit call. While open, requests skip the Redis round-trip entirely
// (immediate L1-only decision) instead of each one blocking on the full
// RedisTimeout during a sustained outage. After cooldown a single probe
// re-checks Redis; success closes it, failure re-opens it.
type circuitBreaker struct {
	mu        sync.Mutex
	threshold int
	cooldown  time.Duration

	failures int
	state    cbState
	openedAt time.Time
}

func newCircuitBreaker(threshold int, cooldown time.Duration) *circuitBreaker {
	return &circuitBreaker{threshold: threshold, cooldown: cooldown, state: cbClosed}
}

// allow reports whether the Redis call should be attempted now.
func (cb *circuitBreaker) allow() bool {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	switch cb.state {
	case cbOpen:
		if time.Since(cb.openedAt) >= cb.cooldown {
			cb.state = cbHalfOpen // permit a probe
			return true
		}
		return false
	case cbHalfOpen:
		return true // probe (concurrent probes are acceptable here)
	default: // cbClosed
		return true
	}
}

// success records a successful Redis call (closes a half-open breaker).
func (cb *circuitBreaker) success() {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.failures = 0
	if cb.state == cbHalfOpen {
		cb.state = cbClosed
	}
}

// failure records a failed Redis call and opens the breaker once the
// consecutive-failure threshold is reached.
func (cb *circuitBreaker) failure() {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.failures++
	switch cb.state {
	case cbHalfOpen:
		cb.state = cbOpen
		cb.openedAt = time.Now()
	case cbClosed:
		if cb.failures >= cb.threshold {
			cb.state = cbOpen
			cb.openedAt = time.Now()
		}
	}
}
