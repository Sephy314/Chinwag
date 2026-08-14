package main

import (
	"os"
	"time"
)

// Alert routing categories. Each category has its own Discord webhook
// (DISCORD_WEBHOOK_URL_<CATEGORY>); "default" (DISCORD_WEBHOOK_URL) is the
// fallback for alerts with no/unknown category. Resolved alerts always go to
// the "recoveries" webhook.
const (
	catDefault     = "default"
	catIncidents   = "incidents"
	catDeployments = "deployments"
	catTraffic     = "traffic"
	catRecoveries  = "recoveries"
	catWarnings    = "warnings"
)

// webhookEnvVars maps a routing category to the env var holding its webhook URL.
var webhookEnvVars = map[string]string{
	catDefault:     "DISCORD_WEBHOOK_URL",
	catIncidents:   "DISCORD_WEBHOOK_URL_INCIDENTS",
	catDeployments: "DISCORD_WEBHOOK_URL_DEPLOYMENTS",
	catTraffic:     "DISCORD_WEBHOOK_URL_TRAFFIC",
	catRecoveries:  "DISCORD_WEBHOOK_URL_RECOVERIES",
	catWarnings:    "DISCORD_WEBHOOK_URL_WARNINGS",
}

// Config holds all runtime configuration for the notifier. Everything is
// read from environment variables so secrets (the Discord webhook URLs) never
// live in code or committed manifests.
type Config struct {
	// Port the HTTP server listens on.
	Port string
	// Webhooks maps a routing category (see webhookEnvVars) to a Discord
	// webhook URL. "default" is the fallback when a category has no URL.
	Webhooks map[string]string
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

	webhooks := make(map[string]string, len(webhookEnvVars))
	for cat, env := range webhookEnvVars {
		if u := os.Getenv(env); u != "" {
			webhooks[cat] = u
		}
	}

	return &Config{
		Port:           port,
		Webhooks:       webhooks,
		DiscordTimeout: timeout,
	}
}
