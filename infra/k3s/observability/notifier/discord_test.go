package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"time"
)

// mockDiscordServer is an in-process stand-in for the Discord webhook API. It
// records every request and can be configured to fail or hang, so tests never
// touch the real network.
type mockDiscordServer struct {
	mu       sync.Mutex
	requests []discordRequest
	status   int
	delay    time.Duration
	ts       *httptest.Server
}

type discordRequest struct {
	Method string
	Path   string
	Body   DiscordMessage
}

func newMockDiscordServer() *mockDiscordServer {
	m := &mockDiscordServer{status: http.StatusNoContent}
	m.ts = httptest.NewServer(http.HandlerFunc(m.handle))
	return m
}

func (m *mockDiscordServer) handle(w http.ResponseWriter, r *http.Request) {
	if m.delay > 0 {
		time.Sleep(m.delay)
	}
	var msg DiscordMessage
	_ = json.NewDecoder(r.Body).Decode(&msg)
	m.mu.Lock()
	m.requests = append(m.requests, discordRequest{Method: r.Method, Path: r.URL.Path, Body: msg})
	status := m.status
	m.mu.Unlock()
	w.WriteHeader(status)
}

// URL returns a webhook URL with a non-trivial path, so tests also verify the
// client keeps the exact path from the configured URL.
func (m *mockDiscordServer) URL() string { return m.ts.URL + "/api/webhooks/123456/token" }

func (m *mockDiscordServer) Count() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.requests)
}

func (m *mockDiscordServer) Last() discordRequest {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.requests) == 0 {
		return discordRequest{}
	}
	return m.requests[len(m.requests)-1]
}

func (m *mockDiscordServer) Close() { m.ts.Close() }
