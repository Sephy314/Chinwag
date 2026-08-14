package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestDiscordSendCorrectRequest(t *testing.T) {
	mock := newMockDiscordServer()
	defer mock.Close()

	client := &DiscordClient{HTTPClient: mock.ts.Client()}
	msg := DiscordMessage{Embeds: []DiscordEmbed{{Title: "🔴 X", Color: colorRed}}}

	if err := client.Send(context.Background(), mock.URL(), msg); err != nil {
		t.Fatalf("Send returned error: %v", err)
	}

	req := mock.Last()
	if req.Method != http.MethodPost {
		t.Errorf("method = %s, want POST", req.Method)
	}
	if req.Path != "/api/webhooks/123456/token" {
		t.Errorf("path = %q, want the full webhook path", req.Path)
	}
	if len(req.Body.Embeds) != 1 || req.Body.Embeds[0].Title != "🔴 X" {
		t.Errorf("decoded body mismatch: %+v", req.Body)
	}
}

func TestDiscordSendErrorOnNon2xx(t *testing.T) {
	mock := newMockDiscordServer()
	defer mock.Close()
	mock.status = http.StatusBadRequest

	client := &DiscordClient{HTTPClient: mock.ts.Client()}
	err := client.Send(context.Background(), mock.URL(), DiscordMessage{})
	if err == nil {
		t.Fatal("expected error for non-2xx response")
	}
	if !strings.Contains(err.Error(), "400") {
		t.Errorf("error should mention status, got %v", err)
	}
}

func TestDiscordSendMissingURL(t *testing.T) {
	client := &DiscordClient{HTTPClient: &http.Client{}}
	err := client.Send(context.Background(), "", DiscordMessage{})
	if err == nil {
		t.Fatal("expected error for missing webhook url")
	}
}

func TestDiscordSendTimeout(t *testing.T) {
	mock := newMockDiscordServer()
	defer mock.Close()
	mock.delay = 300 * time.Millisecond

	client := &DiscordClient{
		HTTPClient: &http.Client{Timeout: 50 * time.Millisecond},
	}
	start := time.Now()
	err := client.Send(context.Background(), mock.URL(), DiscordMessage{})
	if err == nil {
		t.Fatal("expected timeout error")
	}
	if time.Since(start) > 200*time.Millisecond {
		t.Errorf("timeout did not bound the call: took %v", time.Since(start))
	}
}

func TestDiscordSendContextDeadline(t *testing.T) {
	mock := newMockDiscordServer()
	defer mock.Close()
	mock.delay = 300 * time.Millisecond

	client := &DiscordClient{HTTPClient: mock.ts.Client()}
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	if err := client.Send(ctx, mock.URL(), DiscordMessage{}); err == nil {
		t.Fatal("expected context deadline error")
	}
}

func TestDiscordSendPayloadIsJSON(t *testing.T) {
	mock := newMockDiscordServer()
	defer mock.Close()

	client := &DiscordClient{HTTPClient: mock.ts.Client()}
	msg := DiscordMessage{Embeds: []DiscordEmbed{{Title: "t", Color: colorGreen, Fields: []DiscordField{{Name: "Service", Value: "auth"}}}}}
	if err := client.Send(context.Background(), mock.URL(), msg); err != nil {
		t.Fatalf("Send returned error: %v", err)
	}

	// The mock already decoded the JSON; verify the round trip is faithful.
	req := mock.Last()
	if len(req.Body.Embeds) != 1 {
		t.Fatalf("expected 1 embed")
	}
	e := req.Body.Embeds[0]
	if e.Title != "t" || e.Color != colorGreen {
		t.Errorf("embed mismatch: %+v", e)
	}
	if len(e.Fields) != 1 || e.Fields[0].Name != "Service" || e.Fields[0].Value != "auth" {
		t.Errorf("field mismatch: %+v", e.Fields)
	}
	// And it marshals to valid JSON that parses.
	raw, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}
	if !json.Valid(raw) {
		t.Errorf("marshaled payload is not valid json")
	}
}

func TestDiscordURLFor(t *testing.T) {
	client := &DiscordClient{
		Webhooks: map[string]string{
			"default":    "https://discord.example/default",
			"incidents":  "https://discord.example/incidents",
			"recoveries": "https://discord.example/recoveries",
		},
	}
	if got := client.URLFor("incidents"); got != "https://discord.example/incidents" {
		t.Errorf("URLFor(incidents) = %q", got)
	}
	// Category without a dedicated webhook falls back to the default.
	if got := client.URLFor("warnings"); got != "https://discord.example/default" {
		t.Errorf("URLFor(warnings) = %q, want default", got)
	}
	// No default configured -> empty.
	only := &DiscordClient{Webhooks: map[string]string{"incidents": "https://discord.example/incidents"}}
	if got := only.URLFor("warnings"); got != "" {
		t.Errorf("URLFor(warnings) with no default = %q, want empty", got)
	}
}

func TestDiscordSendErrorIncludesBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"code":50035,"message":"Invalid Form Body: embed.fields"}`))
	}))
	defer srv.Close()

	client := &DiscordClient{HTTPClient: srv.Client()}
	err := client.Send(context.Background(), srv.URL, DiscordMessage{})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "status 400") {
		t.Errorf("error should include the HTTP status, got: %v", err)
	}
	if !strings.Contains(err.Error(), "Invalid Form Body") {
		t.Errorf("error should include the Discord response body, got: %v", err)
	}
}

func TestDiscordHasWebhooks(t *testing.T) {
	if (&DiscordClient{Webhooks: map[string]string{}}).HasWebhooks() {
		t.Error("empty webhooks should report false")
	}
	if !(&DiscordClient{Webhooks: map[string]string{"default": "https://discord.example/x"}}).HasWebhooks() {
		t.Error("a configured webhook should report true")
	}
	if (&DiscordClient{Webhooks: map[string]string{"incidents": ""}}).HasWebhooks() {
		t.Error("empty-value webhook should report false")
	}
}
