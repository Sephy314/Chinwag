package main

import (
	"fmt"
	"strings"
	"time"
)

// Discord embed colors (Discord's integer color codes).
const (
	colorRed    = 0xED4245
	colorGreen  = 0x57F287
	colorYellow = 0xFEE75C
)

// maxEmbedsPerMessage is Discord's per-message embed limit. Alertmanager may
// batch many alerts into one webhook notification; we cap the embeds so a
// single notification can never exceed the Discord API limit.
const maxEmbedsPerMessage = 10

// fieldValueLimit is Discord's per-field value length limit (1024 chars).
const fieldValueLimit = 1024

// DiscordMessage is the payload sent to a Discord webhook.
type DiscordMessage struct {
	Content string         `json:"content,omitempty"`
	Embeds  []DiscordEmbed `json:"embeds,omitempty"`
}

// DiscordEmbed is a single rich embed inside a Discord message.
type DiscordEmbed struct {
	Title       string         `json:"title,omitempty"`
	Description string         `json:"description,omitempty"`
	Color       int            `json:"color,omitempty"`
	URL         string         `json:"url,omitempty"`
	Timestamp   string         `json:"timestamp,omitempty"`
	Fields      []DiscordField `json:"fields,omitempty"`
}

// DiscordField is a name/value row inside an embed.
type DiscordField struct {
	Name   string `json:"name"`
	Value  string `json:"value"`
	Inline bool   `json:"inline"`
}

// BuildMessage converts an Alertmanager webhook payload into a single Discord
// message. One embed is generated per alert (capped at maxEmbedsPerMessage), so
// a batched notification produces one message instead of Discord spam.
func BuildMessage(p *AlertmanagerPayload) DiscordMessage {
	msg := DiscordMessage{Embeds: []DiscordEmbed{}}
	if p == nil {
		return msg
	}

	payloadStatus := p.Status
	if payloadStatus == "" {
		payloadStatus = "firing"
	}

	alerts := p.Alerts
	if len(alerts) > maxEmbedsPerMessage {
		alerts = alerts[:maxEmbedsPerMessage]
	}

	for i := range alerts {
		msg.Embeds = append(msg.Embeds, buildEmbed(&alerts[i], payloadStatus))
	}
	return msg
}

// buildEmbed renders a single alert as one Discord embed. Missing labels are
// never required: absent values are simply omitted from the fields.
func buildEmbed(a *Alert, payloadStatus string) DiscordEmbed {
	if a == nil {
		a = &Alert{}
	}

	status := a.Status
	if status == "" {
		status = payloadStatus
	}
	resolved := status == "resolved"

	alertname := labelValue(a, "alertname", "Unknown Alert")

	summary := annotationValue(a, "summary")
	if summary == "" {
		summary = annotationValue(a, "description")
	}
	if summary == "" {
		summary = "No summary provided"
	}

	color := colorRed
	title := "🔴 " + alertname
	if resolved {
		color = colorGreen
		title = "🟢 " + alertname + " (resolved)"
	}

	embed := DiscordEmbed{
		Title:       title,
		Description: truncate(summary, fieldValueLimit),
		Color:       color,
		URL:         a.GeneratorURL,
		Fields:      buildFields(a, resolved),
	}
	if !a.StartsAt.IsZero() {
		embed.Timestamp = a.StartsAt.UTC().Format(time.RFC3339)
	}
	return embed
}

// buildFields collects the readable label/annotation rows for an embed.
func buildFields(a *Alert, resolved bool) []DiscordField {
	var fields []DiscordField
	add := func(name, value string, inline bool) {
		if value == "" {
			return
		}
		fields = append(fields, DiscordField{Name: name, Value: truncate(value, fieldValueLimit), Inline: inline})
	}

	add("Service", labelValue(a, "service", ""), true)
	add("Severity", labelValue(a, "severity", ""), true)
	add("Instance", labelValue(a, "instance", ""), false)
	add("Pod", labelValue(a, "pod", ""), true)
	add("Namespace", labelValue(a, "namespace", ""), true)
	add("Started", formatTime(a.StartsAt), false)
	if resolved {
		add("Resolved", formatTime(a.EndsAt), true)
		add("Duration", humanizeDuration(a.EndsAt.Sub(a.StartsAt)), true)
	}
	return fields
}

// labelValue reads a label, falling back when missing or empty.
func labelValue(a *Alert, key, fallback string) string {
	if a.Labels != nil {
		if v := a.Labels[key]; v != "" {
			return v
		}
	}
	return fallback
}

// annotationValue reads an annotation, returning "" when missing.
func annotationValue(a *Alert, key string) string {
	if a.Annotations != nil {
		return a.Annotations[key]
	}
	return ""
}

// formatTime renders a time as RFC3339 UTC, or "" for the zero value.
func formatTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339)
}

// humanizeDuration renders a duration compactly but unambiguously,
// e.g. "10m 0s", "1h 2m 3s", "1d 1h 30m 0s".
func humanizeDuration(d time.Duration) string {
	if d < 0 {
		d = -d
	}
	if d == 0 {
		return "0s"
	}
	days := int(d / (24 * time.Hour))
	d -= time.Duration(days) * 24 * time.Hour
	hours := int(d / time.Hour)
	d -= time.Duration(hours) * time.Hour
	mins := int(d / time.Minute)
	d -= time.Duration(mins) * time.Minute
	secs := int(d / time.Second)

	var parts []string
	if days > 0 {
		parts = append(parts, fmt.Sprintf("%dd", days))
	}
	if hours > 0 {
		parts = append(parts, fmt.Sprintf("%dh", hours))
	}
	if mins > 0 {
		parts = append(parts, fmt.Sprintf("%dm", mins))
	}
	// Always include seconds so the duration is unambiguous.
	parts = append(parts, fmt.Sprintf("%ds", secs))
	return strings.Join(parts, " ")
}

// truncate limits a string to max runes, appending an ellipsis.
func truncate(s string, max int) string {
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	if max <= 1 {
		return "…"
	}
	return string(r[:max-1]) + "…"
}
