package service

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Sephy314/chinwag/backend/shared/auth/dpop"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func padded(b []byte, size int) []byte {
	if len(b) >= size {
		return b[:size]
	}
	out := make([]byte, size)
	copy(out[size-len(b):], b)
	return out
}

// signDPoPProof signs a compact JWS the same way a real DPoP client would
// (RFC 9449, ES256), so dpop.Validator can parse and verify it.
func signDPoPProof(t *testing.T, priv *ecdsa.PrivateKey, header, payload map[string]any) string {
	t.Helper()
	hb, err := json.Marshal(header)
	require.NoError(t, err)
	pb, err := json.Marshal(payload)
	require.NoError(t, err)

	enc := base64.RawURLEncoding
	signingInput := enc.EncodeToString(hb) + "." + enc.EncodeToString(pb)
	digest := sha256.Sum256([]byte(signingInput))
	r, s, err := ecdsa.Sign(rand.Reader, priv, digest[:])
	require.NoError(t, err)
	sig := append(r.FillBytes(make([]byte, 32)), s.FillBytes(make([]byte, 32))...)

	return signingInput + "." + enc.EncodeToString(sig)
}

// makeValidProof returns a signed DPoP proof bound to a fresh key with the
// given htu/nonce, plus its private key.
func makeValidProof(t *testing.T, htu, nonce string) string {
	t.Helper()
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	size := (priv.PublicKey.Curve.Params().BitSize + 7) / 8
	enc := base64.RawURLEncoding
	jwkJSON := `{"kty":"EC","crv":"P-256","x":"` +
		enc.EncodeToString(padded(priv.PublicKey.X.Bytes(), size)) +
		`","y":"` +
		enc.EncodeToString(padded(priv.PublicKey.Y.Bytes(), size)) + `"}`

	header := map[string]any{
		"typ": "dpop+jwt",
		"alg": "ES256",
		"jwk": json.RawMessage(jwkJSON),
	}
	payload := map[string]any{
		"jti":   uuid.NewString(),
		"htm":   "POST",
		"htu":   htu,
		"iat":   time.Now().Unix(),
		"nonce": nonce,
	}
	return signDPoPProof(t, priv, header, payload)
}

const dpopTestHTU = "http://localhost:8081/auth/login"

func TestDPoPService_Validate_Success(t *testing.T) {
	cache := new(MockCache)
	svc := NewDPoPService(cache)

	raw := makeValidProof(t, dpopTestHTU, "nonce-1")
	req := httptest.NewRequest(http.MethodPost, dpopTestHTU, nil)
	req.Header.Set(dpop.HeaderName, raw)

	cache.On("ConsumeNonce", mock.Anything, "nonce-1").Return(true, nil).Once()
	cache.On("ReserveJti", mock.Anything, mock.Anything, mock.Anything).Return(true, nil).Once()
	cache.On("Set", mock.Anything, mock.MatchedBy(func(k string) bool {
		return len(k) > len("dpop:nonce:")
	}), mock.Anything, dpopNonceTTL).Return(nil).Once()

	proof, nonce, err := svc.Validate(context.Background(), req)

	assert.NoError(t, err)
	assert.NotNil(t, proof)
	assert.NotEmpty(t, nonce)
	cache.AssertExpectations(t)
}

// TestDPoPService_Validate_RedisDown_NonceIssueFails: proof validates fine but
// issuing the fresh nonce hits a dead Redis → Validate must surface the error.
func TestDPoPService_Validate_RedisDown_NonceIssueFails(t *testing.T) {
	cache := new(MockCache)
	svc := NewDPoPService(cache)

	raw := makeValidProof(t, dpopTestHTU, "nonce-1")
	req := httptest.NewRequest(http.MethodPost, dpopTestHTU, nil)
	req.Header.Set(dpop.HeaderName, raw)

	cache.On("ConsumeNonce", mock.Anything, "nonce-1").Return(true, nil).Once()
	cache.On("ReserveJti", mock.Anything, mock.Anything, mock.Anything).Return(true, nil).Once()
	cache.On("Set", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(errors.New("redis connection refused")).Once()

	proof, nonce, err := svc.Validate(context.Background(), req)

	assert.Nil(t, proof)
	assert.Empty(t, nonce)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "redis connection refused")
	cache.AssertExpectations(t)
}

// TestDPoPService_Validate_MissingProof_WithRedisDown: when Redis is also down,
// the missing-proof path cannot even issue a challenge nonce — it still returns
// ErrMissingProof, just without a nonce.
func TestDPoPService_Validate_MissingProof_WithRedisDown(t *testing.T) {
	cache := new(MockCache)
	svc := NewDPoPService(cache)

	cache.On("Set", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(errors.New("redis connection refused")).Once()

	req := httptest.NewRequest(http.MethodPost, dpopTestHTU, nil)

	proof, nonce, err := svc.Validate(context.Background(), req)

	assert.Nil(t, proof)
	assert.Empty(t, nonce)
	assert.ErrorIs(t, err, dpop.ErrMissingProof)
	cache.AssertExpectations(t)
}

func TestDPoPService_Validator_IsWired(t *testing.T) {
	cache := new(MockCache)
	svc := NewDPoPService(cache)

	v := svc.Validator()

	assert.NotNil(t, v)
	assert.NotNil(t, v.NonceStore)
	assert.NotNil(t, v.IssueNonce)
}
