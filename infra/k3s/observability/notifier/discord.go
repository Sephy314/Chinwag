package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// DiscordClient posts messages to a Discord webhook. It is deliberately
// dependency-free (stdlib http only) and keeps the webhook URL in memory only —
// it is never logged.
type DiscordClient struct {
	// WebhookURL is the Discord webhook endpoint to POST to.
	WebhookURL string
	// HTTPClient performs the request; give it a timeout to bound the call.
	HTTPClient *http.Client
}

// Send POSTs msg to the configured webhook and returns an error on any
// transport or non-2xx response.
func (c *DiscordClient) Send(ctx context.Context, msg DiscordMessage) error {
	if c.WebhookURL == "" {
		return fmt.Errorf("discord webhook url is not configured")
	}

	body, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("encode discord message: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.WebhookURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create discord request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "chinwag-notifier/1.0")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("send discord webhook: %w", err)
	}
	defer resp.Body.Close()

	// Drain a little of the body so the connection can be reused; Discord
	// returns an error message in the body on 4xx that we log only as status.
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return fmt.Errorf("discord webhook returned status %d", resp.StatusCode)
	}
	return nil
}
