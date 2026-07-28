package main

import (
	"os"
	"strings"
)

type Config struct {
	Port        string
	Services    map[string]string
	StripPrefix []string
	Default     string
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

	var stripPrefix []string
	if sp := os.Getenv("STRIP_PREFIX"); sp != "" {
		stripPrefix = strings.Split(sp, ",")
	}

	return &Config{
		Port:        port,
		Services:    services,
		StripPrefix: stripPrefix,
		Default:     defaultURL,
	}
}
