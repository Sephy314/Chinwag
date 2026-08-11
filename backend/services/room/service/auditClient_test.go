package service

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAuditClient_DisabledWhenURLEmpty(t *testing.T) {
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	c := NewAuditClient("", "", "", "", log)
	assert.NotNil(t, c)

	// Record with no URL should be a no-op (no panic).
	c.Record(context.Background(), "admin1", "room.create", "room", "r1", nil)
}

func TestAuditClient_Record_SendsPayload(t *testing.T) {
	var got map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/internal/audit", r.URL.Path)
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		_ = json.NewDecoder(r.Body).Decode(&got)
		w.WriteHeader(http.StatusCreated)
	}))
	defer srv.Close()

	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	c := NewAuditClient(srv.URL+"/internal/audit", "", "", "", log)
	c.Record(context.Background(), "admin1", "room.create", "room", "r1", map[string]any{"name": "General"})

	require.NotNil(t, got)
	assert.Equal(t, "admin1", got["admin_id"])
	assert.Equal(t, "room.create", got["action"])
	assert.Equal(t, "room", got["target_type"])
	assert.Equal(t, "r1", got["target_id"])
}

func TestAuditClient_Record_ServerError_NoPanic(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	c := NewAuditClient(srv.URL, "", "", "", log)
	c.Record(context.Background(), "admin1", "room.delete", "room", "r1", nil)
}

func TestAuditClient_Record_ServerDown_NoPanic(t *testing.T) {
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	c := NewAuditClient("http://127.0.0.1:1", "", "", "", log)
	c.Record(context.Background(), "admin1", "room.delete", "room", "r1", nil)
}
