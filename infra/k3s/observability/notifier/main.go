package main

import (
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/joho/godotenv"
)

func main() {
	// Load a local .env if present (dev convenience). In Kubernetes the values
	// come from the chinwag-notifier-secrets Secret / Deployment env instead.
	_ = godotenv.Load()

	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))

	cfg := LoadConfig()

	discord := &DiscordClient{
		Webhooks:   cfg.Webhooks,
		HTTPClient: &http.Client{Timeout: cfg.DiscordTimeout},
	}
	if !discord.HasWebhooks() {
		slog.Warn("no Discord webhook configured (DISCORD_WEBHOOK_URL / DISCORD_WEBHOOK_URL_*) — alerts will not be delivered (health still served)")
	}
	h := NewHandler(cfg, discord)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", h.Health)
	mux.HandleFunc("POST /webhooks/alertmanager", h.AlertmanagerWebhook)

	srv := &http.Server{
		Addr:              "0.0.0.0:" + cfg.Port,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	slog.Info("chinwag-notifier starting", "port", cfg.Port)
	if err := srv.ListenAndServe(); err != nil {
		slog.Error("server failed", "error", err)
		os.Exit(1)
	}
}
