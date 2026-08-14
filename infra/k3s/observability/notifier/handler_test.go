package main

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// newTestHandler returns a Handler wired to the given Discord client, using a
// sane default config unless overridden.
func newTestHandler(discord *DiscordClient, cfg *Config) *Handler {
	if cfg == nil {
		cfg = &Config{Port: "9095", DiscordTimeout: 2 * time.Second}
	}
	return NewHandler(cfg, discord)
}

// defaultClient wires a Discord client with only the default webhook, so every
// category falls back to it.
func defaultClient(m *mockDiscordServer) *DiscordClient {
	return &DiscordClient{
		Webhooks:   map[string]string{"default": m.WebhookURL("default")},
		HTTPClient: m.ts.Client(),
	}
}

// categoryClient wires a Discord client with dedicated webhooks per category
// plus the default fallback.
func categoryClient(m *mockDiscordServer) *DiscordClient {
	return &DiscordClient{
		Webhooks: map[string]string{
			"default":     m.WebhookURL("default"),
			"incidents":   m.WebhookURL("incidents"),
			"deployments": m.WebhookURL("deployments"),
			"traffic":     m.WebhookURL("traffic"),
			"recoveries":  m.WebhookURL("recoveries"),
			"warnings":    m.WebhookURL("warnings"),
		},
		HTTPClient: m.ts.Client(),
	}
}

func postPayload(t *testing.T, h *Handler, raw string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/webhooks/alertmanager", bytes.NewBufferString(raw))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.AlertmanagerWebhook(rec, req)
	return rec
}

func firingPayload() string {
	return `{
	  "status": "firing",
	  "alerts": [
	    {
	      "status": "firing",
	      "labels": {
	        "alertname": "AuthDown",
	        "service": "auth",
	        "severity": "critical",
	        "instance": "auth:8081",
	        "pod": "auth-abc123",
	        "namespace": "chinwag"
	      },
	      "annotations": {"summary": "Auth service is down"},
	      "startsAt": "2026-08-14T10:00:00Z",
	      "endsAt": "0001-01-01T00:00:00Z",
	      "generatorURL": "http://prometheus.example/graph?g0.expr=up"
	    }
	  ],
	  "groupLabels": {"alertname": "AuthDown"},
	  "commonLabels": {"severity": "critical"}
	}`
}

func resolvedPayload() string {
	return `{
	  "status": "resolved",
	  "alerts": [
	    {
	      "status": "resolved",
	      "labels": {"alertname": "AuthDown", "service": "auth", "severity": "critical", "namespace": "chinwag"},
	      "annotations": {"summary": "Auth service recovered"},
	      "startsAt": "2026-08-14T09:55:00Z",
	      "endsAt": "2026-08-14T10:05:00Z"
	    }
	  ]
	}`
}

func TestAlertmanagerWebhookFiring(t *testing.T) {
	discord := newMockDiscordServer()
	defer discord.Close()

	h := newTestHandler(defaultClient(discord), nil)
	rec := postPayload(t, h, firingPayload())

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (body: %s)", rec.Code, rec.Body.String())
	}
	if n := discord.Count(); n != 1 {
		t.Fatalf("expected 1 discord call, got %d", n)
	}
	req := discord.Last()
	if req.Method != http.MethodPost {
		t.Errorf("expected POST to discord, got %s", req.Method)
	}
	if !strings.HasSuffix(req.Path, "/api/webhooks/123456/default") {
		t.Errorf("expected full webhook path, got %q", req.Path)
	}
	if len(req.Body.Embeds) != 1 {
		t.Fatalf("expected 1 embed, got %d", len(req.Body.Embeds))
	}
	e := req.Body.Embeds[0]
	if e.Title != "🔴 AuthDown" {
		t.Errorf("expected firing title, got %q", e.Title)
	}
	if e.Description != "Auth service is down" {
		t.Errorf("expected summary as description, got %q", e.Description)
	}
	if e.Color != colorRed {
		t.Errorf("expected red color, got %d", e.Color)
	}
	if !hasField(e, "Service", "auth") || !hasField(e, "Severity", "critical") || !hasField(e, "Instance", "auth:8081") || !hasField(e, "Namespace", "chinwag") {
		t.Errorf("expected service/severity/instance/namespace fields, got %+v", e.Fields)
	}
	if !strings.Contains(e.Title, "AuthDown") {
		t.Errorf("title should contain alertname")
	}
}

func TestAlertmanagerWebhookResolved(t *testing.T) {
	discord := newMockDiscordServer()
	defer discord.Close()

	h := newTestHandler(defaultClient(discord), nil)
	rec := postPayload(t, h, resolvedPayload())

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	e := discord.Last().Body.Embeds[0]
	if e.Title != "🟢 AuthDown (resolved)" {
		t.Errorf("expected resolved title, got %q", e.Title)
	}
	if e.Color != colorGreen {
		t.Errorf("expected green color, got %d", e.Color)
	}
	if !hasField(e, "Resolved", "2026-08-14T10:05:00Z") {
		t.Errorf("expected Resolved field, got %+v", e.Fields)
	}
	if !hasField(e, "Duration", "10m 0s") {
		t.Errorf("expected Duration field, got %+v", e.Fields)
	}
}

func TestAlertmanagerWebhookInvalidJSON(t *testing.T) {
	discord := newMockDiscordServer()
	defer discord.Close()

	h := newTestHandler(defaultClient(discord), nil)
	rec := postPayload(t, h, `this is not json {`)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
	if n := discord.Count(); n != 0 {
		t.Errorf("discord must not be called for invalid json, got %d calls", n)
	}
}

func TestAlertmanagerWebhookTrailingData(t *testing.T) {
	discord := newMockDiscordServer()
	defer discord.Close()

	h := newTestHandler(defaultClient(discord), nil)
	rec := postPayload(t, h, firingPayload()+"\n{extra:true}")

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for trailing data, got %d", rec.Code)
	}
	if n := discord.Count(); n != 0 {
		t.Errorf("discord must not be called, got %d calls", n)
	}
}

func TestAlertmanagerWebhookEmptyAlerts(t *testing.T) {
	discord := newMockDiscordServer()
	defer discord.Close()

	h := newTestHandler(defaultClient(discord), nil)
	rec := postPayload(t, h, `{"status":"firing","alerts":[]}`)

	// An empty batch is a valid no-op, not an error.
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for empty alerts, got %d", rec.Code)
	}
	if n := discord.Count(); n != 0 {
		t.Errorf("discord must not be called for empty alerts, got %d calls", n)
	}
}

func TestAlertmanagerWebhookMissingWebhookURL(t *testing.T) {
	discord := newMockDiscordServer()
	defer discord.Close()

	h := newTestHandler(&DiscordClient{HTTPClient: discord.ts.Client()}, nil)
	rec := postPayload(t, h, firingPayload())

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 when webhook not configured, got %d", rec.Code)
	}
	if n := discord.Count(); n != 0 {
		t.Errorf("discord must not be called without a webhook url, got %d calls", n)
	}
}

func TestAlertmanagerWebhookDiscordError(t *testing.T) {
	discord := newMockDiscordServer()
	defer discord.Close()
	discord.status = http.StatusInternalServerError

	h := newTestHandler(defaultClient(discord), nil)
	rec := postPayload(t, h, firingPayload())

	if rec.Code != http.StatusBadGateway {
		t.Fatalf("expected 502 on discord failure, got %d", rec.Code)
	}
	if n := discord.Count(); n != 1 {
		t.Fatalf("expected discord to be attempted once, got %d", n)
	}
}

func TestAlertmanagerWebhookDiscordTimeout(t *testing.T) {
	discord := newMockDiscordServer()
	defer discord.Close()
	discord.delay = 200 * time.Millisecond

	// The HTTP client itself must time out (50ms) so the send fails fast.
	client := &http.Client{Timeout: 50 * time.Millisecond}
	cfg := &Config{Port: "9095", DiscordTimeout: 2 * time.Second}
	h := newTestHandler(&DiscordClient{Webhooks: map[string]string{"default": discord.WebhookURL("default")}, HTTPClient: client}, cfg)
	rec := postPayload(t, h, firingPayload())

	if rec.Code != http.StatusBadGateway {
		t.Fatalf("expected 502 on discord timeout, got %d", rec.Code)
	}
}

func TestAlertmanagerWebhookSparseAlert(t *testing.T) {
	discord := newMockDiscordServer()
	defer discord.Close()

	h := newTestHandler(defaultClient(discord), nil)
	// Alert with only an alertname — no labels, annotations or timestamps.
	rec := postPayload(t, h, `{
	  "status": "firing",
	  "alerts": [{"labels": {"alertname": "Mystery"}}]
	}`)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for sparse alert, got %d (body: %s)", rec.Code, rec.Body.String())
	}
	e := discord.Last().Body.Embeds[0]
	if e.Title != "🔴 Mystery" {
		t.Errorf("expected title from alertname, got %q", e.Title)
	}
	if e.Description != "No summary provided" {
		t.Errorf("expected fallback description, got %q", e.Description)
	}
}

func TestAlertmanagerWebhookUnknownFieldsIgnored(t *testing.T) {
	discord := newMockDiscordServer()
	defer discord.Close()

	h := newTestHandler(defaultClient(discord), nil)
	// Extra unknown labels/annotations and JSON keys must not fail the payload.
	rec := postPayload(t, h, `{
	  "status": "firing",
	  "weirdFutureField": {"nested": true},
	  "alerts": [{
	    "labels": {"alertname": "X", "service": "gateway", "someUnknownLabel": "zzz"},
	    "annotations": {"summary": "ok", "futureAnnotation": "y"},
	    "unknownTopLevel": 42
	  }]
	}`)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 with unknown fields, got %d", rec.Code)
	}
	e := discord.Last().Body.Embeds[0]
	if e.Description != "ok" {
		t.Errorf("expected summary, got %q", e.Description)
	}
}

func TestAlertmanagerWebhookRoutesByCategory(t *testing.T) {
	discord := newMockDiscordServer()
	defer discord.Close()

	h := newTestHandler(categoryClient(discord), nil)
	rec := postPayload(t, h, `{
	  "status": "firing",
	  "alerts": [
	    {"labels": {"alertname": "GatewayDown", "service": "gateway", "category": "incidents"}},
	    {"labels": {"alertname": "ReplicasMismatch", "category": "deployments"}},
	    {"labels": {"alertname": "TrafficSpike", "service": "gateway", "category": "traffic"}},
	    {"labels": {"alertname": "HighCPU", "service": "auth", "category": "warnings"}},
	    {"labels": {"alertname": "NoCategory"}}
	  ]
	}`)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (body: %s)", rec.Code, rec.Body.String())
	}
	if n := discord.Count(); n != 5 {
		t.Fatalf("expected 5 discord calls (one per category), got %d", n)
	}
	paths := map[string]bool{}
	for _, r := range discord.All() {
		paths[r.Path] = true
	}
	for _, want := range []string{"incidents", "deployments", "traffic", "warnings", "default"} {
		if !paths["/api/webhooks/123456/"+want] {
			t.Errorf("expected a request to the %q webhook, got paths %v", want, paths)
		}
	}
}

func TestAlertmanagerWebhookResolvedGoesToRecoveries(t *testing.T) {
	discord := newMockDiscordServer()
	defer discord.Close()

	h := newTestHandler(categoryClient(discord), nil)
	rec := postPayload(t, h, `{
	  "status": "resolved",
	  "alerts": [
	    {"status": "resolved", "labels": {"alertname": "GatewayDown", "service": "gateway", "category": "incidents"}}
	  ]
	}`)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if n := discord.Count(); n != 1 {
		t.Fatalf("expected 1 discord call, got %d", n)
	}
	if !strings.HasSuffix(discord.Last().Path, "/recoveries") {
		t.Errorf("resolved alert should go to the recoveries webhook, got path %q", discord.Last().Path)
	}
}

func TestAlertmanagerWebhookCategoryFallsBackToDefault(t *testing.T) {
	discord := newMockDiscordServer()
	defer discord.Close()

	// Only the default webhook is configured — "deployments" must fall back.
	h := newTestHandler(defaultClient(discord), nil)
	rec := postPayload(t, h, `{
	  "status": "firing",
	  "alerts": [{"labels": {"alertname": "ReplicasMismatch", "category": "deployments"}}]
	}`)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if n := discord.Count(); n != 1 {
		t.Fatalf("expected 1 discord call, got %d", n)
	}
	if !strings.HasSuffix(discord.Last().Path, "/default") {
		t.Errorf("expected fallback to default webhook, got path %q", discord.Last().Path)
	}
}

func hasField(e DiscordEmbed, name, value string) bool {
	for _, f := range e.Fields {
		if f.Name == name && f.Value == value {
			return true
		}
	}
	return false
}
