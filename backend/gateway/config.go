package main

import "os"

type Config struct {
	Port     string
	Services map[string]string
	Default  string
}

func LoadConfig() *Config {
	port := os.Getenv("GATEWAY_PORT")
	if port == "" {
		port = "8000"
	}

	services := make(map[string]string)

	if authURL := os.Getenv("AUTH_SERVICE_URL"); authURL != "" {
		services["/auth"] = authURL
	}
	if roomURL := os.Getenv("ROOM_SERVICE_URL"); roomURL != "" {
		services["/rooms"] = roomURL
		services["/users"] = roomURL
	}
	if chatURL := os.Getenv("CHAT_SERVICE_URL"); chatURL != "" {
		services["/chat"] = chatURL
	}

	defaultURL := os.Getenv("DEFAULT_SERVICE_URL")

	return &Config{
		Port:     port,
		Services: services,
		Default:  defaultURL,
	}
}
