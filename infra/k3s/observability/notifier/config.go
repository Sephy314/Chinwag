package main

import (
	"os"
	"time"
)

// Config holds all runtime configuration for the notifier. Everything is
// read from environment variables so secrets (the Discord webhook URL) never
// live in code or committed manifests.
type Config struct {
	// Port the HTTP server listens on.
	Port string
	// DiscordWebhook is the Discord webhook URL (from DISCORD_WEBHOOK_URL).
	// In Kubernetes it is injected via a Secret; it is never logged.
	DiscordWebhook string
	// DiscordTimeout bounds how long a single Discord webhook POST may take.
	DiscordTimeout time.Duration
}

// LoadConfig reads configuration from the environment, applying small
// defaults so the service runs out of the box locally.
func LoadConfig() *Config {
	port := os.Getenv("NOTIFIER_PORT")
	if port == "" {
		port = "9095"
	}

	timeout := 10 * time.Second
	if v := os.Getenv("NOTIFIER_DISCORD_TIMEOUT"); v != "" {
		if d, err := time.ParseDuration(v); err == nil && d > 0 {
			timeout = d
		}
	}

	return &Config{
		Port:           port,
		DiscordWebhook: os.Getenv("DISCORD_WEBHOOK_URL"),
		DiscordTimeout: timeout,
	}
}
