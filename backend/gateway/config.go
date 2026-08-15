package main

import (
	"os"
	"strconv"
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
	// the backend services.
	RateLimitEnabled bool
	RateLimitRate    float64
	RateLimitBurst   int
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
		Port:             port,
		Routes:           routes,
		RateLimitEnabled: rateLimitEnabled,
		RateLimitRate:    rateLimitRate,
		RateLimitBurst:   rateLimitBurst,
	}
}
