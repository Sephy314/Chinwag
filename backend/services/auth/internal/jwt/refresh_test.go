package jwt

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSignVerifyRefreshToken_RoundTrip(t *testing.T) {
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)

	now := time.Now()
	raw, err := SignRefreshToken("u1", "jti1", "sid1", "jkt-abc", priv, "ref-kid", now, time.Hour)
	require.NoError(t, err)

	claims, kid, err := VerifyRefreshToken(raw, &priv.PublicKey, now)
	require.NoError(t, err)
	assert.Equal(t, "u1", claims.Subject)
	assert.Equal(t, "jti1", claims.JTI)
	assert.Equal(t, "sid1", claims.SID)
	assert.Equal(t, "jkt-abc", claims.Jkt)
	assert.Equal(t, "ref-kid", kid)
}

func TestVerifyRefreshToken_Tampered(t *testing.T) {
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)

	raw, err := SignRefreshToken("u1", "jti1", "sid1", "", priv, "ref-kid", time.Now(), time.Hour)
	require.NoError(t, err)

	tampered := raw[:len(raw)-3] + "abc"
	_, _, err = VerifyRefreshToken(tampered, &priv.PublicKey, time.Now())
	assert.Error(t, err)
}
