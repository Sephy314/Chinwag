package dpop

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"encoding/json"
	"net/http/httptest"
	"testing"
	"time"
)

func mustKey(t *testing.T) (*ecdsa.PrivateKey, json.RawMessage) {
	t.Helper()
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	size := (priv.PublicKey.Curve.Params().BitSize + 7) / 8
	jwkJSON := `{"kty":"EC","crv":"P-256","x":"` + b64.EncodeToString(paddedBytes(priv.PublicKey.X.Bytes(), size)) +
		`","y":"` + b64.EncodeToString(paddedBytes(priv.PublicKey.Y.Bytes(), size)) + `"}`
	return priv, json.RawMessage(jwkJSON)
}

func signProof(t *testing.T, priv *ecdsa.PrivateKey, header, payload map[string]any) string {
	t.Helper()
	hb, err := json.Marshal(header)
	if err != nil {
		t.Fatalf("marshal header: %v", err)
	}
	pb, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	signingInput := b64.EncodeToString(hb) + "." + b64.EncodeToString(pb)
	digest := sha256.Sum256([]byte(signingInput))
	r, s, err := ecdsa.Sign(rand.Reader, priv, digest[:])
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	sig := append(r.FillBytes(make([]byte, 32)), s.FillBytes(make([]byte, 32))...)
	return signingInput + "." + b64.EncodeToString(sig)
}

func validProof(t *testing.T, priv *ecdsa.PrivateKey, pubJwk json.RawMessage, overrides map[string]any) string {
	t.Helper()
	payload := map[string]any{
		"jti": "test-jti",
		"htm": "POST",
		"htu": "http://localhost:3000/auth/refresh",
		"iat": time.Now().Unix(),
	}
	for k, v := range overrides {
		payload[k] = v
	}
	return signProof(t, priv, map[string]any{
		"typ": "dpop+jwt",
		"alg": "ES256",
		"jwk": json.RawMessage(pubJwk),
	}, payload)
}

func TestParseProofAndVerify(t *testing.T) {
	priv, pubJwk := mustKey(t)
	raw := validProof(t, priv, pubJwk, nil)

	proof, err := ParseProof(raw)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if proof.Claims.Jti != "test-jti" {
		t.Errorf("jti = %q", proof.Claims.Jti)
	}
	if err := proof.VerifySignature(); err != nil {
		t.Fatalf("verify: %v", err)
	}

	now := time.Now()
	if err := proof.Validate("POST", "http://localhost:3000/auth/refresh", now, time.Minute, time.Minute); err != nil {
		t.Fatalf("validate: %v", err)
	}
}

func TestParseProofRejectsBadTypAlg(t *testing.T) {
	priv, pubJwk := mustKey(t)
	raw := validProof(t, priv, pubJwk, nil)

	parsed, err := ParseProof(raw)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	parsed.Header.Typ = "jwt"
	raw2 := signProof(t, priv, map[string]any{
		"typ": "jwt",
		"alg": "ES256",
		"jwk": json.RawMessage(pubJwk),
	}, map[string]any{"jti": "x"})
	_ = parsed

	if _, err := ParseProof(raw2); err == nil {
		t.Fatal("expected error for typ != dpop+jwt")
	}

	raw3 := signProof(t, priv, map[string]any{
		"typ": "dpop+jwt",
		"alg": "RS256",
		"jwk": json.RawMessage(pubJwk),
	}, map[string]any{"jti": "x"})
	if _, err := ParseProof(raw3); err == nil {
		t.Fatal("expected error for alg != ES256")
	}
}

func TestValidateRejects(t *testing.T) {
	priv, pubJwk := mustKey(t)
	now := time.Now()

	cases := []struct {
		name      string
		overrides map[string]any
		htu       string
		method    string
		now       time.Time
	}{
		{"missing jti", map[string]any{"jti": ""}, "http://localhost:3000/auth/refresh", "POST", now},
		{"htm mismatch", nil, "http://localhost:3000/auth/refresh", "GET", now},
		{"htu mismatch", nil, "http://evil.example/refresh", "POST", now},
		{"iat future", map[string]any{"iat": now.Add(2 * time.Minute).Unix()}, "http://localhost:3000/auth/refresh", "POST", now},
		{"iat expired", map[string]any{"iat": now.Add(-2 * time.Minute).Unix()}, "http://localhost:3000/auth/refresh", "POST", now},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			raw := validProof(t, priv, pubJwk, tc.overrides)
			proof, err := ParseProof(raw)
			if err != nil {
				t.Fatalf("parse: %v", err)
			}
			if err := proof.Validate(tc.method, tc.htu, tc.now, time.Minute, time.Minute); err == nil {
				t.Fatal("expected validation error")
			}
		})
	}
}

func TestThumbprint(t *testing.T) {
	priv, _ := mustKey(t)
	jkt1, err := Thumbprint(&priv.PublicKey)
	if err != nil {
		t.Fatalf("thumbprint: %v", err)
	}

	other, _ := mustKey(t)
	jkt2, err := Thumbprint(&other.PublicKey)
	if err != nil {
		t.Fatalf("thumbprint: %v", err)
	}

	if jkt1 == jkt2 {
		t.Fatal("thumbprints should differ for different keys")
	}

	proof, err := ParseProof(validProof(t, priv, jwkFor(t, priv), nil))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	jkt3, err := proof.Thumbprint()
	if err != nil {
		t.Fatalf("proof thumbprint: %v", err)
	}
	if jkt1 != jkt3 {
		t.Fatalf("proof thumbprint = %q, want %q", jkt3, jkt1)
	}
}

func jwkFor(t *testing.T, priv *ecdsa.PrivateKey) json.RawMessage {
	t.Helper()
	size := (priv.PublicKey.Curve.Params().BitSize + 7) / 8
	return json.RawMessage(`{"kty":"EC","crv":"P-256","x":"` +
		b64.EncodeToString(paddedBytes(priv.PublicKey.X.Bytes(), size)) +
		`","y":"` + b64.EncodeToString(paddedBytes(priv.PublicKey.Y.Bytes(), size)) + `"}`)
}

func TestRequestHTU(t *testing.T) {
	r := httptest.NewRequest("POST", "/refresh", nil)
	r.Header.Set("X-Forwarded-Proto", "https")
	r.Header.Set("X-Forwarded-Host", "chinwag.com")
	r.Header.Set("X-Forwarded-Prefix", "/auth")

	if got := RequestHTU(r); got != "https://chinwag.com/auth/refresh" {
		t.Fatalf("RequestHTU = %q", got)
	}

	r2 := httptest.NewRequest("POST", "/refresh", nil)
	r2.Host = "localhost:8081"
	if got := RequestHTU(r2); got != "http://localhost:8081/refresh" {
		t.Fatalf("RequestHTU no-forwarded = %q", got)
	}
}
