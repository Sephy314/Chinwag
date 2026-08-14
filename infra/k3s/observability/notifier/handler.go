package main

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"sort"
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

	if !h.discord.HasWebhooks() {
		slog.Error("discord webhooks are not configured — set DISCORD_WEBHOOK_URL (and/or DISCORD_WEBHOOK_URL_*)")
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "discord webhook not configured"})
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), h.cfg.DiscordTimeout)
	defer cancel()

	// Route alerts by category (incidents/deployments/traffic/recoveries/
	// warnings) so each category can be delivered to its own Discord webhook.
	groups := groupByCategory(&p)
	cats := make([]string, 0, len(groups))
	for cat := range groups {
		cats = append(cats, cat)
	}
	sort.Strings(cats)

	sent, failed := 0, 0
	for _, cat := range cats {
		alerts := groups[cat]
		url := h.discord.URLFor(cat)
		if url == "" {
			// No dedicated webhook for this category and no default — skip.
			slog.Warn("no webhook for alert category, skipping", "category", cat, "alerts", len(alerts))
			continue
		}
		sub := &AlertmanagerPayload{Status: p.Status, Alerts: alerts, ExternalURL: p.ExternalURL}
		msg := BuildMessage(sub)
		if err := h.discord.Send(ctx, url, msg); err != nil {
			// Do NOT log the webhook URL or any part of it.
			slog.Error("discord send failed", "error", err, "category", cat, "alerts", len(alerts))
			failed++
			continue
		}
		sent++
		slog.Info("alertmanager webhook delivered to discord",
			"category", cat, "alerts", len(alerts), "status", p.Status)
	}

	if sent == 0 {
		if failed > 0 {
			writeJSON(w, http.StatusBadGateway, map[string]string{"error": "failed to deliver notification to discord"})
		} else {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "no webhook configured for alert categories"})
		}
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "alerts": len(p.Alerts)})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
