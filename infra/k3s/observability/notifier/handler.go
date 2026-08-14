package main

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
)

// maxBodyBytes bounds the size of an incoming webhook payload.
const maxBodyBytes = 1 << 20 // 1 MiB

// Handler serves the notifier's HTTP endpoints.
type Handler struct {
	cfg     *Config
	discord *DiscordClient
}

// NewHandler builds a Handler with its dependencies.
func NewHandler(cfg *Config, discord *DiscordClient) *Handler {
	return &Handler{cfg: cfg, discord: discord}
}

// Health is a liveness/readiness endpoint (200 = healthy).
func (h *Handler) Health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// AlertmanagerWebhook receives Alertmanager webhook notifications and forwards
// them to Discord. It does not judge alert conditions — Alertmanager already
// decided those — it only parses, formats and delivers.
//
//	2xx: accepted and delivered
//	4xx: malformed / unprocessable payload
//	5xx: internal error (e.g. Discord send failure, missing webhook config)
func (h *Handler) AlertmanagerWebhook(w http.ResponseWriter, r *http.Request) {
	body := http.MaxBytesReader(w, r.Body, maxBodyBytes)
	dec := json.NewDecoder(body)

	var p AlertmanagerPayload
	if err := dec.Decode(&p); err != nil {
		slog.Warn("invalid alertmanager payload", "error", err)
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json payload"})
		return
	}
	// Reject trailing garbage after the JSON document.
	if err := dec.Decode(&struct{}{}); err != io.EOF {
		slog.Warn("alertmanager payload has trailing data")
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "unexpected trailing data"})
		return
	}

	if len(p.Alerts) == 0 {
		// Valid no-op: Alertmanager can emit an empty batch (e.g. an empty
		// resolved notification). Nothing to notify — not an error.
		slog.Debug("alertmanager webhook with no alerts", "status", p.Status)
		writeJSON(w, http.StatusOK, map[string]string{"status": "no alerts to notify"})
		return
	}

	if h.discord.WebhookURL == "" {
		slog.Error("discord webhook is not configured — set DISCORD_WEBHOOK_URL")
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "discord webhook not configured"})
		return
	}

	msg := BuildMessage(&p)
	ctx, cancel := context.WithTimeout(r.Context(), h.cfg.DiscordTimeout)
	defer cancel()

	if err := h.discord.Send(ctx, msg); err != nil {
		// Do NOT log the webhook URL or any part of it.
		slog.Error("discord send failed", "error", err, "status", p.Status, "alerts", len(p.Alerts))
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "failed to deliver notification to discord"})
		return
	}

	slog.Info("alertmanager webhook delivered to discord",
		"status", p.Status, "alerts", len(p.Alerts))
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "alerts": len(p.Alerts)})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
