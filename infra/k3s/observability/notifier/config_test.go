package main

import (
	"testing"
	"time"
)

func TestLoadConfigWebhooks(t *testing.T) {
	t.Setenv("DISCORD_WEBHOOK_URL", "https://discord.example/default")
	t.Setenv("DISCORD_WEBHOOK_URL_INCIDENTS", "https://discord.example/incidents")
	t.Setenv("DISCORD_WEBHOOK_URL_DEPLOYMENTS", "https://discord.example/deployments")
	t.Setenv("DISCORD_WEBHOOK_URL_TRAFFIC", "https://discord.example/traffic")
	t.Setenv("DISCORD_WEBHOOK_URL_RECOVERIES", "https://discord.example/recoveries")
	t.Setenv("DISCORD_WEBHOOK_URL_WARNINGS", "https://discord.example/warnings")

	cfg := LoadConfig()

	want := map[string]string{
		catDefault:     "https://discord.example/default",
		catIncidents:   "https://discord.example/incidents",
		catDeployments: "https://discord.example/deployments",
		catTraffic:     "https://discord.example/traffic",
		catRecoveries:  "https://discord.example/recoveries",
		catWarnings:    "https://discord.example/warnings",
	}
	for cat, url := range want {
		if cfg.Webhooks[cat] != url {
			t.Errorf("Webhooks[%q] = %q, want %q", cat, cfg.Webhooks[cat], url)
		}
	}
}

func TestLoadConfigDefaults(t *testing.T) {
	// Ensure a clean environment so defaults are deterministic.
	for _, env := range webhookEnvVars {
		t.Setenv(env, "")
	}
	t.Setenv("NOTIFIER_PORT", "")
	t.Setenv("NOTIFIER_DISCORD_TIMEOUT", "")

	cfg := LoadConfig()

	if cfg.Port != "9095" {
		t.Errorf("Port = %q, want default 9095", cfg.Port)
	}
	if cfg.DiscordTimeout != 10*time.Second {
		t.Errorf("DiscordTimeout = %v, want 10s", cfg.DiscordTimeout)
	}
	if len(cfg.Webhooks) != 0 {
		t.Errorf("Webhooks should be empty with no env, got %v", cfg.Webhooks)
	}
}
