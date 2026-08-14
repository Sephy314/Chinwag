package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// DiscordClient posts messages to Discord webhooks. It keeps the webhook URLs
// in memory only — they are never logged.
type DiscordClient struct {
	// Webhooks maps a routing category (or "default") to a Discord webhook URL.
	Webhooks map[string]string
	// HTTPClient performs the requests; give it a timeout to bound the calls.
	HTTPClient *http.Client
}

// HasWebhooks reports whether at least one webhook URL is configured.
func (c *DiscordClient) HasWebhooks() bool {
	for _, u := range c.Webhooks {
		if u != "" {
			return true
		}
	}
	return false
}

// URLFor returns the webhook URL for a category, falling back to the "default"
// webhook URL when the category has none. Returns "" when neither is set.
func (c *DiscordClient) URLFor(category string) string {
	if u := c.Webhooks[category]; u != "" {
		return u
	}
	return c.Webhooks[catDefault]
}

// Send POSTs msg to the given webhook URL and returns an error on any
// transport or non-2xx response.
func (c *DiscordClient) Send(ctx context.Context, url string, msg DiscordMessage) error {
	if url == "" {
		return fmt.Errorf("discord webhook url is not configured")
	}

	body, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("encode discord message: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
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

	// Capture a little of the response body. Discord returns a JSON error on
	// 4xx (e.g. Invalid Form Body: <field>) — including it in the error makes
	// the exact reason visible in the logs. It never contains the webhook URL.
	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
	// Drain the rest so the connection can be reused (HTTP keep-alive) under
	// alert bursts; leaving bytes unread forces a new connection each time.
	_, _ = io.Copy(io.Discard, resp.Body)

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		errMsg := fmt.Sprintf("discord webhook returned status %d", resp.StatusCode)
		if body := strings.TrimSpace(string(respBody)); body != "" {
			errMsg += ": " + body
		}
		return fmt.Errorf("%s", errMsg)
	}
	return nil
}
