package middleware

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// Rate-limit observability. `mode` distinguishes the decision source:
//   - "redis" — authoritative distributed decision
//   - "l1"    — local in-memory decision (fast path, or degraded during a
//     Redis outage / memory backend)
var (
	rateLimitAllowed = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "rate_limit_allowed_total",
		Help: "Total requests allowed by the gateway rate limiter, by decision mode.",
	}, []string{"mode"})

	rateLimitRejected = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "rate_limit_rejected_total",
		Help: "Total requests rejected (429) by the gateway rate limiter, by decision mode.",
	}, []string{"mode"})

	rateLimitRedisErrors = promauto.NewCounter(prometheus.CounterOpts{
		Name: "rate_limit_redis_errors_total",
		Help: "Total Redis limiter errors (connection/timeout/command) that triggered an L1 fallback.",
	})

	rateLimitL1Fallback = promauto.NewCounter(prometheus.CounterOpts{
		Name: "rate_limit_l1_fallback_total",
		Help: "Total requests served by the L1 in-memory limiter because Redis was unavailable.",
	})

	rateLimitRedisLatency = promauto.NewHistogram(prometheus.HistogramOpts{
		Name:    "rate_limit_redis_latency_seconds",
		Help:    "Redis rate-limit round-trip latency.",
		Buckets: prometheus.DefBuckets,
	})
)
