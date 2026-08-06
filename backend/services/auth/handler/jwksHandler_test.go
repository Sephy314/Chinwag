package handler

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"errors"
	"net/http"
	"testing"

	"github.com/labstack/echo/v5/echotest"
	"github.com/lestrrat-go/jwx/v3/jwk"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func makeJwkSet(t *testing.T) jwk.Set {
	t.Helper()
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	key, err := jwk.Import(&priv.PublicKey)
	require.NoError(t, err)
	_ = key.Set(jwk.KeyIDKey, "kid1")
	set := jwk.NewSet()
	require.NoError(t, set.AddKey(key))
	return set
}

func TestJwksHandler_ServeJWKS_Success(t *testing.T) {
	jwks := new(MockJwksService)
	h := NewJwksHandler(jwks)

	set := makeJwkSet(t)
	jwks.On("GetJwkSet", mock.Anything).Return(set, nil).Once()

	c, rec := echotest.ContextConfig{}.ToContextRecorder(t)
	err := h.ServeJWKS(c)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.NotEmpty(t, rec.Body.Bytes())
	jwks.AssertExpectations(t)
}

func TestJwksHandler_ServeJWKS_Error(t *testing.T) {
	jwks := new(MockJwksService)
	h := NewJwksHandler(jwks)

	jwks.On("GetJwkSet", mock.Anything).Return(nil, errors.New("jwks unavailable")).Once()

	c, _ := echotest.ContextConfig{}.ToContextRecorder(t)
	err := h.ServeJWKS(c)
	assert.EqualError(t, err, "jwks unavailable")
	jwks.AssertExpectations(t)
}
