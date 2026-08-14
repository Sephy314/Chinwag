package main

import (
	"log/slog"
	"net/http"
	"os"
	"time"
)

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))

	cfg := LoadConfig()

	if cfg.DiscordWebhook == "" {
		slog.Warn("DISCORD_WEBHOOK_URL is not set — alerts will not be delivered (health still served)")
	}

	discord := &DiscordClient{
		WebhookURL: cfg.DiscordWebhook,
		HTTPClient: &http.Client{Timeout: cfg.DiscordTimeout},
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
