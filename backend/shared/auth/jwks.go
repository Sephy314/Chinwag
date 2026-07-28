package auth

import (
	"context"
	"crypto/ecdsa"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/lestrrat-go/jwx/v3/jwk"
)

type JWKSClient struct {
	jwksURL  string
	cacheTTL time.Duration
	set      jwk.Set
	mu       sync.RWMutex
	loadedAt time.Time
	http     *http.Client
}

func NewJWKSClient(jwksURL string, cacheTTL time.Duration) *JWKSClient {
	return &JWKSClient{
		jwksURL:  jwksURL,
		cacheTTL: cacheTTL,
		set:      jwk.NewSet(),
		http:     &http.Client{Timeout: 5 * time.Second},
	}
}

func (c *JWKSClient) KeyFunc() jwt.Keyfunc {
	return func(token *jwt.Token) (any, error) {
		if _, ok := token.Method.(*jwt.SigningMethodECDSA); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}

		kid, ok := token.Header["kid"].(string)
		if !ok {
			return nil, fmt.Errorf("missing kid in token header")
		}

		return c.GetKey(kid)
	}
}

func (c *JWKSClient) expired() bool {
	return time.Since(c.loadedAt) > c.cacheTTL
}

func (c *JWKSClient) exportKey(key jwk.Key) (*ecdsa.PublicKey, error) {
	var pub ecdsa.PublicKey
	if err := jwk.Export(key, &pub); err != nil {
		return nil, fmt.Errorf("failed to export key: %w", err)
	}
	return &pub, nil
}

func (c *JWKSClient) refresh() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.expired() {
		return nil
	}

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, c.jwksURL, nil)
	if err != nil {
		return err
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("JWKS endpoint returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	var raw struct {
		Keys []map[string]any `json:"keys"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return err
	}

	set := jwk.NewSet()
	for _, rawKey := range raw.Keys {
		keyBytes, err := json.Marshal(rawKey)
		if err != nil {
			return err
		}

		parsedKey, err := jwk.ParseKey(keyBytes)
		if err != nil {
			return err
		}

		if err := set.AddKey(parsedKey); err != nil {
			return err
		}
	}

	c.set = set
	c.loadedAt = time.Now()
	return nil
}

func (c *JWKSClient) Invalidate() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.set = jwk.NewSet()
	c.loadedAt = time.Time{}
}

func (c *JWKSClient) ParseToken(tokenString string, claims jwt.Claims) (*jwt.Token, error) {
	return jwt.ParseWithClaims(tokenString, claims, c.KeyFunc())
}

func (c *JWKSClient) GetKey(kid string) (*ecdsa.PublicKey, error) {
	c.mu.RLock()
	if key, ok := c.set.LookupKeyID(kid); ok && !c.expired() {
		c.mu.RUnlock()
		return c.exportKey(key)
	}
	c.mu.RUnlock()

	if err := c.refresh(); err != nil {
		return nil, fmt.Errorf("failed to refresh JWKS: %w", err)
	}

	c.mu.RLock()
	defer c.mu.RUnlock()

	key, ok := c.set.LookupKeyID(kid)
	if !ok {
		return nil, fmt.Errorf("key %s not found in JWKS", kid)
	}

	return c.exportKey(key)
}
