package service

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"log/slog"
	"net/http"
	"os"
	"time"
)

// AuditClient writes admin audit events to the auth service's internal mTLS
// endpoint (POST /internal/audit). It is best-effort: a failure to record is
// logged but does not fail the administrative operation.
type AuditClient struct {
	url        string
	httpClient *http.Client
	log        *slog.Logger
}

func NewAuditClient(url, clientCert, clientKey, caFile string, log *slog.Logger) *AuditClient {
	if log == nil {
		log = slog.Default()
	}
	if url == "" {
		return &AuditClient{log: log}
	}

	tlsCfg := &tls.Config{MinVersion: tls.VersionTLS12}
	switch {
	case clientCert != "" && clientKey != "":
		cert, err := tls.LoadX509KeyPair(clientCert, clientKey)
		if err != nil {
			log.Warn("audit: failed to load client cert", "cert", clientCert, "error", err)
		} else {
			tlsCfg.Certificates = []tls.Certificate{cert}
		}
	case clientCert != "" || clientKey != "":
		log.Warn("audit: only one of client cert/key set; mTLS client certificate disabled",
			"client_cert_set", clientCert != "", "client_key_set", clientKey != "")
	}
	if caFile != "" {
		caPEM, err := os.ReadFile(caFile)
		if err != nil {
			log.Warn("audit: failed to read CA file", "path", caFile, "error", err)
		} else {
			pool := x509.NewCertPool()
			if !pool.AppendCertsFromPEM(caPEM) {
				log.Warn("audit: failed to parse CA PEM", "path", caFile)
			} else {
				tlsCfg.RootCAs = pool
			}
		}
	}

	return &AuditClient{
		url: url,
		httpClient: &http.Client{
			Timeout:   5 * time.Second,
			Transport: &http.Transport{TLSClientConfig: tlsCfg},
		},
		log: log,
	}
}

func (c *AuditClient) Record(ctx context.Context, adminID, action, targetType, targetID string, metadata map[string]any) {
	if c.url == "" {
		return
	}
	payload := map[string]any{
		"admin_id":    adminID,
		"action":      action,
		"target_type": targetType,
		"target_id":   targetID,
		"metadata":    metadata,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		c.log.Warn("audit: marshal failed", "error", err)
		return
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.url, bytes.NewReader(body))
	if err != nil {
		c.log.Warn("audit: request build failed", "error", err)
		return
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		c.log.Warn("audit: send failed", "error", err)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		c.log.Warn("audit: unexpected status", "status", resp.StatusCode)
	}
}
