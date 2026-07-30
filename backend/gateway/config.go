package main

import (
	"os"
	"strings"
)

type ServiceRoute struct {
	Prefix    string
	Methods   []string
	TargetURL string
	StripPrefix bool
}

type Config struct {
	Port    string
	Routes  []ServiceRoute
	Default string
}

func LoadConfig() *Config {
	port := os.Getenv("GATEWAY_PORT")
	if port == "" {
		port = "8000"
	}

	var routes []ServiceRoute

	if authURL := os.Getenv("AUTH_SERVICE_URL"); authURL != "" {
		routes = append(routes, ServiceRoute{Prefix: "/auth", TargetURL: authURL})
	}

	roomURL := os.Getenv("ROOM_SERVICE_URL")
	if roomURL != "" {
		routes = append(routes, ServiceRoute{Prefix: "/rooms", TargetURL: roomURL})
		routes = append(routes, ServiceRoute{Prefix: "/users", TargetURL: roomURL})
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
			Prefix:    "/chat/rooms/:roomId/ws",
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

	defaultURL := os.Getenv("DEFAULT_SERVICE_URL")

	var stripPrefix []string
	if sp := os.Getenv("STRIP_PREFIX"); sp != "" {
		stripPrefix = strings.Split(sp, ",")
	}
	_ = stripPrefix

	return &Config{
		Port:    port,
		Routes:  routes,
		Default: defaultURL,
	}
}
