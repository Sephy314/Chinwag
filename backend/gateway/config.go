package main

import "os"

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
}

func LoadConfig() *Config {
	port := os.Getenv("GATEWAY_PORT")
	if port == "" {
		port = "8000"
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
		Port:   port,
		Routes: routes,
	}
}
