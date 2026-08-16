package main

import (
	"os"
	"strconv"
	"time"
)

type ServiceRoute struct {
	Prefix      string
	Suffix      string // optional, path must also end with this
	Methods     []string
	TargetURL   string
	StripPrefix bool
}

type Config struct {
	Port   string
	Routes []ServiceRoute

	// Per-IP rate limiting (token bucket). The gateway is the single edge for
	// all HTTP traffic, so this throttles abusive clients before they reach
	// the backend services. Redis is the authoritative distributed state
	// (shared across gateway replicas); an L1 in-memory limiter is the fast
	// path and the Redis-outage fallback.
	RateLimitEnabled bool
	RateLimitRate    float64
	RateLimitBurst   int

	RateLimitBackend string        // "redis" (default when Redis is configured) | "memory" (L1 only)
	RedisAddr        string        // e.g. "redis:6379" (from GATEWAY_REDIS_ADDR / REDIS_ADDR)
	RedisPassword    string        // cluster Redis requires auth
	RedisKeyTTL      time.Duration // per-IP rate-limit key TTL (idle cleanup)
	RedisTimeout     time.Duration // bounds a single Redis round-trip
	L1ExpiresIn      time.Duration // idle L1 bucket expiry

	// TrustedProxyCIDR is the only proxy hop whose X-Forwarded-For is trusted
	// (Traefik's source — the k3s pod CIDR). Everything else is treated as the
	// client, so an internal client can't spoof XFF to rotate rate-limit keys.
	TrustedProxyCIDR string

	// Redis circuit breaker: after RedisCircuitThreshold consecutive failures
	// within an outage, the breaker opens and requests skip the Redis round-trip
	// (immediate L1-only) instead of each paying the full RedisTimeout. A probe
	// after RedisCircuitCooldown re-checks Redis and closes on success.
	RedisCircuitThreshold int
	RedisCircuitCooldown  time.Duration
}

func LoadConfig() *Config {
	port := os.Getenv("GATEWAY_PORT")
	if port == "" {
		port = "8000"
	}

	// Rate limiting is on by default (generous limits: 20 req/s per IP, burst
	// 60). Disable with GATEWAY_RATE_LIMIT_ENABLED=false, or tune the numbers
	// via env without a rebuild.
	rateLimitEnabled := os.Getenv("GATEWAY_RATE_LIMIT_ENABLED") != "false"
	rateLimitRate := 20.0
	if v := os.Getenv("GATEWAY_RATE_LIMIT_RATE"); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil && f > 0 {
			rateLimitRate = f
		}
	}
	rateLimitBurst := 60
	if v := os.Getenv("GATEWAY_RATE_LIMIT_BURST"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			rateLimitBurst = n
		}
	}

	// Redis (authoritative distributed state). Empty address → memory backend.
	redisAddr := os.Getenv("GATEWAY_REDIS_ADDR")
	if redisAddr == "" {
		redisAddr = os.Getenv("REDIS_ADDR")
	}
	redisPassword := os.Getenv("GATEWAY_REDIS_PASSWORD")
	if redisPassword == "" {
		redisPassword = os.Getenv("REDIS_PASSWORD")
	}
	if redisPassword == "" {
		redisPassword = os.Getenv("REDIS_PW")
	}

	// Backend: explicit GATEWAY_RATE_LIMIT_BACKEND wins; otherwise use Redis
	// when an address is configured (k8s) and fall back to memory for local
	// dev (no Redis → would otherwise add a 50ms timeout to every request).
	rateLimitBackend := os.Getenv("GATEWAY_RATE_LIMIT_BACKEND")
	if rateLimitBackend == "" {
		if redisAddr != "" {
			rateLimitBackend = "redis"
		} else {
			rateLimitBackend = "memory"
		}
	}

	redisKeyTTL := 120 * time.Second
	if v := os.Getenv("GATEWAY_RATE_LIMIT_REDIS_TTL"); v != "" {
		if d, err := time.ParseDuration(v); err == nil && d > 0 {
			redisKeyTTL = d
		}
	}
	redisTimeout := 50 * time.Millisecond
	if v := os.Getenv("GATEWAY_RATE_LIMIT_REDIS_TIMEOUT"); v != "" {
		if d, err := time.ParseDuration(v); err == nil && d > 0 {
			redisTimeout = d
		}
	}
	l1ExpiresIn := 5 * time.Minute
	if v := os.Getenv("GATEWAY_RATE_LIMIT_L1_EXPIRES"); v != "" {
		if d, err := time.ParseDuration(v); err == nil && d > 0 {
			l1ExpiresIn = d
		}
	}

	// Traefik → Gateway traffic originates from the k3s pod CIDR (Traefik pod
	// IPs). Default to the k3s default pod CIDR; override for other setups.
	trustedProxyCIDR := os.Getenv("GATEWAY_TRUSTED_PROXY_CIDR")
	if trustedProxyCIDR == "" {
		trustedProxyCIDR = "10.42.0.0/16"
	}

	redisCircuitThreshold := 3
	if v := os.Getenv("GATEWAY_RATE_LIMIT_REDIS_CIRCUIT_THRESHOLD"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			redisCircuitThreshold = n
		}
	}
	redisCircuitCooldown := 5 * time.Second
	if v := os.Getenv("GATEWAY_RATE_LIMIT_REDIS_CIRCUIT_COOLDOWN"); v != "" {
		if d, err := time.ParseDuration(v); err == nil && d > 0 {
			redisCircuitCooldown = d
		}
	}

	var routes []ServiceRoute

	if authURL := os.Getenv("AUTH_SERVICE_URL"); authURL != "" {
		routes = append(routes, ServiceRoute{Prefix: "/auth", TargetURL: authURL, StripPrefix: true})
	}

	roomURL := os.Getenv("ROOM_SERVICE_URL")
	if roomURL != "" {
		routes = append(routes, ServiceRoute{Prefix: "/rooms", TargetURL: roomURL})
		routes = append(routes, ServiceRoute{Prefix: "/users", TargetURL: roomURL})
		// Admin panel routes live at the room service root (/admin/...).
		routes = append(routes, ServiceRoute{Prefix: "/admin/rooms", TargetURL: roomURL})
		routes = append(routes, ServiceRoute{Prefix: "/admin/users/", TargetURL: roomURL})
		routes = append(routes, ServiceRoute{Prefix: "/admin/stats/rooms", TargetURL: roomURL})
	}

	chatCommandURL := os.Getenv("CHAT_COMMAND_SERVICE_URL")
	if chatCommandURL == "" {
		chatCommandURL = os.Getenv("CHAT_SERVICE_URL")
	}
	chatQueryURL := os.Getenv("CHAT_QUERY_SERVICE_URL")
	if chatQueryURL == "" {
		chatQueryURL = chatCommandURL
	}

	if chatCommandURL != "" {
		routes = append(routes, ServiceRoute{
			Prefix:    "/chat",
			Methods:   []string{"POST", "PUT", "DELETE"},
			TargetURL: chatCommandURL,
		})
		routes = append(routes, ServiceRoute{
			Prefix:    "/chat/rooms/",
			Suffix:    "/ws",
			Methods:   []string{"GET"},
			TargetURL: chatCommandURL,
		})
	}

	if chatQueryURL != "" {
		routes = append(routes, ServiceRoute{
			Prefix:    "/chat",
			Methods:   []string{"GET"},
			TargetURL: chatQueryURL,
		})
	}

	return &Config{
		Port:                  port,
		Routes:                routes,
		RateLimitEnabled:      rateLimitEnabled,
		RateLimitRate:         rateLimitRate,
		RateLimitBurst:        rateLimitBurst,
		RateLimitBackend:      rateLimitBackend,
		RedisAddr:             redisAddr,
		RedisPassword:         redisPassword,
		RedisKeyTTL:           redisKeyTTL,
		RedisTimeout:          redisTimeout,
		L1ExpiresIn:           l1ExpiresIn,
		TrustedProxyCIDR:      trustedProxyCIDR,
		RedisCircuitThreshold: redisCircuitThreshold,
		RedisCircuitCooldown:  redisCircuitCooldown,
	}
}
