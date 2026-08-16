package middleware

import (
	"testing"
	"time"
)

func TestCircuitBreakerOpensAfterThreshold(t *testing.T) {
	cb := newCircuitBreaker(3, time.Hour)
	if !cb.allow() {
		t.Fatal("should allow while closed")
	}
	cb.failure()
	cb.failure()
	if !cb.allow() {
		t.Fatal("should still allow below the threshold")
	}
	cb.failure() // 3rd consecutive failure → open
	if cb.allow() {
		t.Fatal("breaker should be open after reaching the threshold")
	}
}

func TestCircuitBreakerProbeClosesOnSuccess(t *testing.T) {
	cb := newCircuitBreaker(2, 30*time.Millisecond)
	cb.failure()
	cb.failure()
	if cb.allow() {
		t.Fatal("breaker should be open")
	}
	// After the cooldown, allow() permits a probe.
	time.Sleep(40 * time.Millisecond)
	if !cb.allow() {
		t.Fatal("should allow a probe after cooldown")
	}
	// Probe success closes the breaker (resets failures).
	cb.success()
	if !cb.allow() {
		t.Fatal("breaker should be closed after a successful probe")
	}
	cb.failure()
	cb.failure()
	if cb.allow() {
		t.Fatal("breaker should re-open after fresh failures")
	}
}

func TestCircuitBreakerProbeFailureReopens(t *testing.T) {
	cb := newCircuitBreaker(1, 20*time.Millisecond)
	cb.failure() // threshold 1 → open
	if cb.allow() {
		t.Fatal("breaker should be open")
	}
	time.Sleep(30 * time.Millisecond)
	if !cb.allow() {
		t.Fatal("should allow a probe after cooldown")
	}
	cb.failure() // probe fails → re-open
	if cb.allow() {
		t.Fatal("breaker should be open again after a failed probe")
	}
}

func TestCircuitBreakerSuccessResetsFailures(t *testing.T) {
	cb := newCircuitBreaker(3, time.Hour)
	cb.failure()
	cb.failure()
	cb.success() // below threshold, success resets the counter
	cb.failure()
	cb.failure()
	if !cb.allow() { // 2 failures since the reset < 3 → still closed → allowed
		t.Fatal("should still allow: failures were reset by the earlier success")
	}
	cb.failure() // now 3 consecutive since the reset → open
	if cb.allow() {
		t.Fatal("breaker should open after 3 consecutive failures")
	}
}
