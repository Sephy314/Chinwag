package main

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/labstack/echo/v5"
	"github.com/stretchr/testify/require"
)

// loadTLSCert loads a client certificate/key pair for mTLS testing.
func loadTLSCert(t *testing.T, certFile, keyFile string) tls.Certificate {
	t.Helper()
	cert, err := tls.LoadX509KeyPair(certFile, keyFile)
	require.NoError(t, err)
	return cert
}

func TestBuildInternalTLSConfig_RequiresClientCert(t *testing.T) {
	if _, err := os.Stat(".mtls/ca.crt"); err != nil {
		t.Skip("dev mTLS certs not present — run scripts/gen-dev-mtls.sh")
	}

	cfg, err := buildInternalTLSConfig(".mtls/server.crt", ".mtls/server.key", ".mtls/ca.crt")
	require.NoError(t, err)
	require.Equal(t, tls.RequireAndVerifyClientCert, cfg.ClientAuth)
	require.NotNil(t, cfg.ClientCAs)
	require.Len(t, cfg.Certificates, 1)
}

func TestInternalMTLS_Handshake(t *testing.T) {
	if _, err := os.Stat(".mtls/ca.crt"); err != nil {
		t.Skip("dev mTLS certs not present — run scripts/gen-dev-mtls.sh")
	}

	tlsCfg, err := buildInternalTLSConfig(".mtls/server.crt", ".mtls/server.key", ".mtls/ca.crt")
	require.NoError(t, err)

	e := echo.New()
	e.POST("/internal/audit", func(c *echo.Context) error {
		return c.JSON(http.StatusCreated, map[string]string{"status": "ok"})
	})

	srv := httptest.NewUnstartedServer(e)
	srv.TLS = tlsCfg
	srv.StartTLS()
	defer srv.Close()

	// --- Client WITHOUT a certificate -> handshake must fail ---
	noCertClient := &http.Client{Transport: &http.Transport{TLSClientConfig: &tls.Config{
		InsecureSkipVerify: true, // only to force a handshake; cert verify happens via ClientAuth
	}}}
	_, err = noCertClient.Post(srv.URL+"/internal/audit", "application/json", nil)
	require.Error(t, err, "request without a client cert should fail the handshake")

	// --- Client WITH a cert signed by the CA -> 201 ---
	caPEM, err := os.ReadFile(".mtls/ca.crt")
	require.NoError(t, err)
	pool := x509.NewCertPool()
	pool.AppendCertsFromPEM(caPEM)
	clientCert := loadTLSCert(t, ".mtls/client.crt", ".mtls/client.key")
	goodClient := &http.Client{Transport: &http.Transport{TLSClientConfig: &tls.Config{
		Certificates: []tls.Certificate{clientCert},
		RootCAs:      pool,
	}}}
	resp, err := goodClient.Post(srv.URL+"/internal/audit", "application/json", nil)
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusCreated, resp.StatusCode)

	var body map[string]string
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	require.Equal(t, "ok", body["status"])
}

func TestInternalMTLS_RejectsForeignClientCert(t *testing.T) {
	if _, err := os.Stat(".mtls/ca.crt"); err != nil {
		t.Skip("dev mTLS certs not present — run scripts/gen-dev-mtls.sh")
	}

	tlsCfg, err := buildInternalTLSConfig(".mtls/server.crt", ".mtls/server.key", ".mtls/ca.crt")
	require.NoError(t, err)

	e := echo.New()
	e.POST("/internal/audit", func(c *echo.Context) error { return c.NoContent(http.StatusCreated) })

	srv := httptest.NewUnstartedServer(e)
	srv.TLS = tlsCfg
	srv.StartTLS()
	defer srv.Close()

	// A client presenting the SERVER cert (not signed by the CA as a client) or
	// an untrusted cert must be rejected. Use the server key/cert as a bogus
	// client identity: it is signed by the same CA, but lacks clientAuth EKU, so
	// verification fails.
	bogus := loadTLSCert(t, ".mtls/server.crt", ".mtls/server.key")
	caPEM, _ := os.ReadFile(".mtls/ca.crt")
	pool := x509.NewCertPool()
	pool.AppendCertsFromPEM(caPEM)
	client := &http.Client{Transport: &http.Transport{TLSClientConfig: &tls.Config{
		Certificates: []tls.Certificate{bogus},
		RootCAs:      pool,
	}}}
	_, err = client.Post(srv.URL+"/internal/audit", "application/json", nil)
	require.Error(t, err, "server cert as client identity should be rejected (no clientAuth EKU)")
}
