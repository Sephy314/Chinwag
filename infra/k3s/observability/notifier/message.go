package main

import (
	"fmt"
	"net/url"
	"strings"
	"time"
	"unicode/utf8"
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

// Discord field/embed length limits (chars). Truncation is rune-count based
// (see truncate), so these are a best-effort safety buffer below Discord's
// hard limits (1024/2048/256). The margin makes a 400 from a rune-heavy value
// (emoji, CJK, astral pairs) much less likely, though it is not a strict
// guarantee at the UTF-16 level.
const (
	fieldValueLimit  = 1000 // Discord hard limit: 1024
	descriptionLimit = 2000 // Discord hard limit: 2048
	titleLimit       = 250  // Discord hard limit: 256
)

// messageCharLimit is Discord's aggregate embed limit: the combined characters
// of title + description + field.name + field.value (+ footer + author) across
// ALL embeds attached to one message must not exceed 6000 (docs.discord.com —
// "Message Resource -> Embed Object -> Embed Limits"). BuildMessage shares a
// single budget across the embeds it generates so a batched notification can
// never trigger a 400 from this aggregate limit.
const messageCharLimit = 6000

// charCount counts characters (runes), matching how Discord measures embed
// limits — they count characters, not bytes.
func charCount(s string) int {
	return utf8.RuneCountInString(s)
}

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

	// Discord caps the combined characters across all embeds in a message at
	// messageCharLimit, so one shared budget is consumed in order: earlier
	// alerts keep the most context, and once it's gone we stop adding embeds
	// rather than risk a 400 from the aggregate limit.
	remaining := messageCharLimit
	for i := range alerts {
		e := buildEmbed(&alerts[i], payloadStatus, &remaining)
		if e.Title == "" && e.Description == "" && len(e.Fields) == 0 {
			break // budget exhausted — an empty embed adds nothing
		}
		msg.Embeds = append(msg.Embeds, e)
	}
	return msg
}

// alertCategory returns the webhook routing category for an alert.
// Resolved alerts always go to the "recoveries" webhook; otherwise the
// alert's `category` label (set by the Prometheus alert rules) is used, and
// alerts with no category fall back to the default webhook.
func alertCategory(a *Alert, payloadStatus string) string {
	status := a.Status
	if status == "" {
		status = payloadStatus
	}
	if status == "resolved" {
		return catRecoveries
	}
	if c := labelValue(a, "category", ""); c != "" {
		return c
	}
	return catDefault
}

// groupByCategory buckets the payload's alerts by webhook routing category so
// each category can be delivered to its own Discord webhook.
func groupByCategory(p *AlertmanagerPayload) map[string][]Alert {
	groups := map[string][]Alert{}
	if p == nil {
		return groups
	}
	payloadStatus := p.Status
	if payloadStatus == "" {
		payloadStatus = "firing"
	}
	for i := range p.Alerts {
		cat := alertCategory(&p.Alerts[i], payloadStatus)
		groups[cat] = append(groups[cat], p.Alerts[i])
	}
	return groups
}

// buildEmbed renders a single alert as one Discord embed. Missing labels are
// never required: absent values are simply omitted from the fields.
func buildEmbed(a *Alert, payloadStatus string, remaining *int) DiscordEmbed {
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

	// Everything below draws from the shared per-message budget (*remaining),
	// so the total characters across all embeds never exceed messageCharLimit.
	embed := DiscordEmbed{Color: color, URL: validURL(a.GeneratorURL)}

	if *remaining > 0 {
		budget := titleLimit
		if *remaining < budget {
			budget = *remaining
		}
		embed.Title = truncate(title, budget)
		*remaining -= charCount(embed.Title)
	}
	if *remaining > 0 {
		budget := descriptionLimit
		if *remaining < budget {
			budget = *remaining
		}
		embed.Description = truncate(summary, budget)
		*remaining -= charCount(embed.Description)
	}
	for _, f := range buildFields(a, resolved) {
		if *remaining <= 0 || charCount(f.Name) >= *remaining {
			break // no budget left for this (or any later) field
		}
		valueBudget := *remaining - charCount(f.Name)
		if valueBudget > fieldValueLimit {
			valueBudget = fieldValueLimit
		}
		v := truncate(f.Value, valueBudget)
		if v == "" {
			break
		}
		embed.Fields = append(embed.Fields, DiscordField{Name: f.Name, Value: v, Inline: f.Inline})
		*remaining -= charCount(f.Name) + charCount(v)
	}

	if !a.StartsAt.IsZero() {
		embed.Timestamp = a.StartsAt.UTC().Format(time.RFC3339)
	}
	return embed
}

// validURL returns s only when it is a valid http(s) URL with a host. Discord
// rejects an embed `url` field that is not a valid URL with a 400, so a
// malformed generatorURL must never be included.
func validURL(s string) string {
	u, err := url.Parse(s)
	if err != nil {
		return ""
	}
	if (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return ""
	}
	return s
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
