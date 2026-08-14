package main

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestBuildMessageFiring(t *testing.T) {
	p := &AlertmanagerPayload{Status: "firing", Alerts: []Alert{{
		Status:       "firing",
		Labels:       map[string]string{"alertname": "High5xxRate", "service": "gateway", "severity": "critical"},
		Annotations:  map[string]string{"summary": "5xx rate is elevated", "description": "details"},
		StartsAt:     time.Date(2026, 8, 14, 10, 0, 0, 0, time.UTC),
		GeneratorURL: "http://prom.example/g",
	}}}

	msg := BuildMessage(p)
	if len(msg.Embeds) != 1 {
		t.Fatalf("expected 1 embed, got %d", len(msg.Embeds))
	}
	e := msg.Embeds[0]
	if e.Title != "🔴 High5xxRate" {
		t.Errorf("title = %q", e.Title)
	}
	if e.Color != colorRed {
		t.Errorf("color = %d", e.Color)
	}
	if e.Description != "5xx rate is elevated" {
		t.Errorf("description = %q", e.Description)
	}
	if e.URL != "http://prom.example/g" {
		t.Errorf("url = %q", e.URL)
	}
	if e.Timestamp != "2026-08-14T10:00:00Z" {
		t.Errorf("timestamp = %q", e.Timestamp)
	}
	if !hasField(e, "Service", "gateway") || !hasField(e, "Severity", "critical") {
		t.Errorf("missing expected fields: %+v", e.Fields)
	}
}

func TestBuildMessageResolved(t *testing.T) {
	start := time.Date(2026, 8, 14, 9, 55, 0, 0, time.UTC)
	end := time.Date(2026, 8, 14, 10, 5, 30, 0, time.UTC)
	p := &AlertmanagerPayload{Status: "resolved", Alerts: []Alert{{
		Status:   "resolved",
		Labels:   map[string]string{"alertname": "High5xxRate", "service": "gateway"},
		StartsAt: start,
		EndsAt:   end,
	}}}

	msg := BuildMessage(p)
	e := msg.Embeds[0]
	if e.Title != "🟢 High5xxRate (resolved)" {
		t.Errorf("title = %q", e.Title)
	}
	if e.Color != colorGreen {
		t.Errorf("color = %d", e.Color)
	}
	if !hasField(e, "Resolved", "2026-08-14T10:05:30Z") {
		t.Errorf("missing Resolved field: %+v", e.Fields)
	}
	if !hasField(e, "Duration", "10m 30s") {
		t.Errorf("missing Duration field: %+v", e.Fields)
	}
}

func TestBuildMessageCapsEmbeds(t *testing.T) {
	var alerts []Alert
	for i := 0; i < 25; i++ {
		alerts = append(alerts, Alert{Labels: map[string]string{"alertname": "A"}})
	}
	msg := BuildMessage(&AlertmanagerPayload{Status: "firing", Alerts: alerts})
	if len(msg.Embeds) != maxEmbedsPerMessage {
		t.Fatalf("expected %d embeds, got %d", maxEmbedsPerMessage, len(msg.Embeds))
	}
}

func TestBuildMessageMissingSummaryUsesFallback(t *testing.T) {
	p := &AlertmanagerPayload{Status: "firing", Alerts: []Alert{{
		Labels: map[string]string{"alertname": "NoSummary"},
	}}}
	msg := BuildMessage(p)
	if msg.Embeds[0].Description != "No summary provided" {
		t.Errorf("description = %q", msg.Embeds[0].Description)
	}
}

func TestBuildMessageDescriptionAnnotationFallback(t *testing.T) {
	p := &AlertmanagerPayload{Status: "firing", Alerts: []Alert{{
		Labels:      map[string]string{"alertname": "Desc"},
		Annotations: map[string]string{"description": "long detail text"},
	}}}
	msg := BuildMessage(p)
	if msg.Embeds[0].Description != "long detail text" {
		t.Errorf("description = %q", msg.Embeds[0].Description)
	}
}

func TestBuildMessageNilPayload(t *testing.T) {
	msg := BuildMessage(nil)
	if len(msg.Embeds) != 0 {
		t.Errorf("expected no embeds for nil payload")
	}
}

func TestBuildMessageFiringDefaultsWhenAlertStatusEmpty(t *testing.T) {
	// Alert without its own status inherits the payload status.
	p := &AlertmanagerPayload{Status: "firing", Alerts: []Alert{{Labels: map[string]string{"alertname": "X"}}}}
	if e := BuildMessage(p).Embeds[0]; e.Color != colorRed {
		t.Errorf("expected red for firing, got %d", e.Color)
	}
	p = &AlertmanagerPayload{Status: "resolved", Alerts: []Alert{{Labels: map[string]string{"alertname": "X"}}}}
	if e := BuildMessage(p).Embeds[0]; e.Color != colorGreen {
		t.Errorf("expected green for resolved, got %d", e.Color)
	}
}

func TestTruncate(t *testing.T) {
	if got := truncate("hello", 10); got != "hello" {
		t.Errorf("truncate short = %q", got)
	}
	long := strings.Repeat("x", 2000)
	if got := truncate(long, 100); len([]rune(got)) != 100 {
		t.Errorf("truncate long length = %d, want 100", len([]rune(got)))
	}
	if !strings.HasSuffix(truncate(long, 100), "…") {
		t.Errorf("truncate long should end with ellipsis")
	}
}

func TestHumanizeDuration(t *testing.T) {
	cases := []struct {
		d    time.Duration
		want string
	}{
		{0, "0s"},
		{3 * time.Second, "3s"},
		{2 * time.Minute, "2m 0s"},
		{1*time.Hour + 2*time.Minute + 3*time.Second, "1h 2m 3s"},
		{25*time.Hour + 30*time.Minute, "1d 1h 30m 0s"},
		{-5 * time.Second, "5s"},
	}
	for _, c := range cases {
		if got := humanizeDuration(c.d); got != c.want {
			t.Errorf("humanizeDuration(%v) = %q, want %q", c.d, got, c.want)
		}
	}
}

func TestFormatTime(t *testing.T) {
	if got := formatTime(time.Time{}); got != "" {
		t.Errorf("zero time should format empty, got %q", got)
	}
	ts := time.Date(2026, 8, 14, 10, 0, 0, 0, time.UTC)
	if got := formatTime(ts); got != "2026-08-14T10:00:00Z" {
		t.Errorf("formatTime = %q", got)
	}
}

func TestAlertCategory(t *testing.T) {
	cases := []struct {
		name          string
		alert         Alert
		payloadStatus string
		want          string
	}{
		{"resolved status wins", Alert{Status: "resolved", Labels: map[string]string{"category": "incidents"}}, "firing", catRecoveries},
		{"resolved from payload", Alert{Labels: map[string]string{"category": "warnings"}}, "resolved", catRecoveries},
		{"explicit category", Alert{Status: "firing", Labels: map[string]string{"category": "traffic"}}, "firing", "traffic"},
		{"no category falls back to default", Alert{Status: "firing", Labels: map[string]string{}}, "firing", catDefault},
		{"unknown category passes through", Alert{Status: "firing", Labels: map[string]string{"category": "custom"}}, "firing", "custom"},
	}
	for _, c := range cases {
		if got := alertCategory(&c.alert, c.payloadStatus); got != c.want {
			t.Errorf("alertCategory(%q) = %q, want %q", c.name, got, c.want)
		}
	}
}

func TestGroupByCategory(t *testing.T) {
	p := &AlertmanagerPayload{Status: "firing", Alerts: []Alert{
		{Status: "firing", Labels: map[string]string{"alertname": "A", "category": "incidents"}},
		{Status: "firing", Labels: map[string]string{"alertname": "B", "category": "incidents"}},
		{Status: "firing", Labels: map[string]string{"alertname": "C", "category": "traffic"}},
		{Status: "firing", Labels: map[string]string{"alertname": "D"}},
		{Status: "resolved", Labels: map[string]string{"alertname": "E", "category": "incidents"}},
	}}

	groups := groupByCategory(p)
	if len(groups["incidents"]) != 2 {
		t.Errorf("incidents = %d alerts, want 2", len(groups["incidents"]))
	}
	if len(groups["traffic"]) != 1 {
		t.Errorf("traffic = %d alerts, want 1", len(groups["traffic"]))
	}
	if len(groups[catDefault]) != 1 {
		t.Errorf("default = %d alerts, want 1", len(groups[catDefault]))
	}
	if len(groups[catRecoveries]) != 1 {
		t.Errorf("recoveries = %d alerts, want 1", len(groups[catRecoveries]))
	}
}

func TestGroupByCategoryNilPayload(t *testing.T) {
	if g := groupByCategory(nil); len(g) != 0 {
		t.Errorf("expected no groups for nil payload, got %d", len(g))
	}
}

func TestValidURL(t *testing.T) {
	cases := []struct{ in, want string }{
		{"http://prometheus.example/graph", "http://prometheus.example/graph"},
		{"https://p.example/g?x=1&y=2", "https://p.example/g?x=1&y=2"},
		{"http://10.42.0.1:9090/graph", "http://10.42.0.1:9090/graph"},
		{"http://[::1]:9090/graph", "http://[::1]:9090/graph"},
		{"http://prometheus-server-0.monitoring.svc.cluster.local/graph", "http://prometheus-server-0.monitoring.svc.cluster.local/graph"},
		{"", ""},
		{"not a url", ""},
		{"ftp://x/y", ""},
		{"http://", ""},                 // no host
		{"http://localhost:9090/x", ""}, // bare hostname — Discord rejects it
		{"http://prometheus-server-0:9090/graph", ""}, // k8s pod hostname — Discord rejects it
		{"javascript:alert(1)", ""},
	}
	for _, c := range cases {
		if got := validURL(c.in); got != c.want {
			t.Errorf("validURL(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestBuildMessageDropsBareHostGeneratorURL(t *testing.T) {
	// Real-world generatorURLs use the bare pod hostname (prometheus-server-0),
	// which Discord's embed url validation rejects with a 400. It must be
	// dropped so the embed stays within Discord's accepted fields.
	p := &AlertmanagerPayload{Status: "firing", Alerts: []Alert{{
		Status:       "firing",
		Labels:       map[string]string{"alertname": "ChinwagPVCLowSpace", "category": "warnings"},
		GeneratorURL: "http://prometheus-server-0:9090/graph?g0.tab=1",
	}}}
	msg := BuildMessage(p)
	if len(msg.Embeds) != 1 {
		t.Fatalf("expected 1 embed, got %d", len(msg.Embeds))
	}
	if msg.Embeds[0].URL != "" {
		t.Errorf("bare-host generatorURL should be dropped, got %q", msg.Embeds[0].URL)
	}
}

// messageCharTotal sums Discord's countable embed text (title + description +
// field names/values) across all embeds — the aggregate Discord measures.
func messageCharTotal(msg DiscordMessage) int {
	total := 0
	for _, e := range msg.Embeds {
		total += len([]rune(e.Title)) + len([]rune(e.Description))
		for _, f := range e.Fields {
			total += len([]rune(f.Name)) + len([]rune(f.Value))
		}
	}
	return total
}

func TestBuildMessageAggregateBudgetSingleWorstCase(t *testing.T) {
	// Max-length title, summary and every label present: without an aggregate
	// budget this single embed (250 + 2000 + 6×1000) alone exceeds Discord's
	// 6000-char message limit.
	p := &AlertmanagerPayload{Status: "firing", Alerts: []Alert{{
		Status: "firing",
		Labels: map[string]string{
			"alertname": strings.Repeat("A", 300),
			"service":   strings.Repeat("s", 300),
			"severity":  strings.Repeat("v", 300),
			"instance":  strings.Repeat("i", 300),
			"pod":       strings.Repeat("p", 300),
			"namespace": strings.Repeat("n", 300),
		},
		Annotations: map[string]string{"summary": strings.Repeat("x", 5000)},
	}}}

	msg := BuildMessage(p)
	if len(msg.Embeds) != 1 {
		t.Fatalf("expected 1 embed, got %d", len(msg.Embeds))
	}
	if total := messageCharTotal(msg); total > messageCharLimit {
		t.Errorf("embed char total %d exceeds Discord limit %d", total, messageCharLimit)
	}
}

func TestBuildMessageAggregateBudgetBatched(t *testing.T) {
	// Worst-case batch: more alerts than the embed cap, each with a max-length
	// summary and every field. The shared budget must keep the whole message
	// within Discord's 6000-char aggregate limit.
	var alerts []Alert
	for i := 0; i < 12; i++ {
		alerts = append(alerts, Alert{
			Status: "firing",
			Labels: map[string]string{
				"alertname": "A",
				"service":   strings.Repeat("s", 100),
				"severity":  "critical",
				"instance":  strings.Repeat("i", 100),
				"pod":       strings.Repeat("p", 100),
				"namespace": strings.Repeat("n", 100),
			},
			Annotations: map[string]string{"summary": strings.Repeat("x", 4000)},
		})
	}

	msg := BuildMessage(&AlertmanagerPayload{Status: "firing", Alerts: alerts})
	if len(msg.Embeds) > maxEmbedsPerMessage {
		t.Errorf("embeds %d > max %d", len(msg.Embeds), maxEmbedsPerMessage)
	}
	if len(msg.Embeds) == 0 {
		t.Fatal("expected at least one embed to be kept")
	}
	if total := messageCharTotal(msg); total > messageCharLimit {
		t.Errorf("message char total %d exceeds Discord limit %d", total, messageCharLimit)
	}
}

func TestBuildMessageAggregateBudgetKeepsFirstEmbeds(t *testing.T) {
	// Earlier alerts keep the most context; the shared budget is consumed in
	// order, so the tail of a batch is trimmed instead of blowing past 6000.
	var alerts []Alert
	for i := 0; i < 4; i++ {
		alerts = append(alerts, Alert{
			Status:      "firing",
			Labels:      map[string]string{"alertname": fmt.Sprintf("Alert%d", i), "service": "gateway"},
			Annotations: map[string]string{"summary": strings.Repeat("d", 2000)},
		})
	}

	msg := BuildMessage(&AlertmanagerPayload{Status: "firing", Alerts: alerts})
	if len(msg.Embeds) == 0 {
		t.Fatal("expected at least one embed")
	}
	if msg.Embeds[0].Title != "🔴 Alert0" {
		t.Errorf("first embed should keep its title, got %q", msg.Embeds[0].Title)
	}
	if total := messageCharTotal(msg); total > messageCharLimit {
		t.Errorf("message char total %d exceeds Discord limit %d", total, messageCharLimit)
	}
	if total := messageCharTotal(msg); total < 1000 {
		t.Errorf("expected meaningful content, total = %d", total)
	}
}
