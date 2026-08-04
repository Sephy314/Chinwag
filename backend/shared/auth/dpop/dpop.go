package dpop

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/lestrrat-go/jwx/v3/jwa"
	"github.com/lestrrat-go/jwx/v3/jwk"
	"github.com/lestrrat-go/jwx/v3/jws"
)

const (
	// HeaderName is the HTTP header that carries the DPoP proof (RFC 9449 section 5).
	HeaderName = "DPoP"
	// NonceHeader is the HTTP response header carrying a fresh nonce (RFC 9449 section 8.3).
	NonceHeader = "DPoP-Nonce"

	// ErrorUseNonce signals the client to retry with a fresh nonce.
	ErrorUseNonce = "use_dpop_nonce"
	// ErrorInvalidProof signals a malformed, wrong, or replayed DPoP proof.
	ErrorInvalidProof = "invalid_dpop_proof"
	// ErrorInvalidBinding signals that the proof key does not match the access token's cnf.jkt.
	ErrorInvalidBinding = "invalid_token_binding"

	typValue = "dpop+jwt"
	algValue = "ES256"
)

var b64 = base64.RawURLEncoding

// Proof is a parsed and validated-claim DPoP proof.
type Proof struct {
	Raw    string
	Header proofHeader
	Claims ProofClaims
	Key    *ecdsa.PublicKey
}

type proofHeader struct {
	Typ string          `json:"typ"`
	Alg string          `json:"alg"`
	Jwk json.RawMessage `json:"jwk"`
}

// ProofClaims are the registered claims of a DPoP proof (RFC 9449 section 4.2).
type ProofClaims struct {
	Jti   string `json:"jti"`
	Htm   string `json:"htm"`
	Htu   string `json:"htu"`
	Iat   int64  `json:"iat"`
	Nonce string `json:"nonce,omitempty"`
}

// ParseProof parses a compact JWS proof and extracts the embedded public key.
func ParseProof(raw string) (*Proof, error) {
	parts := strings.Split(raw, ".")
	if len(parts) != 3 {
		return nil, errors.New("invalid compact JWS")
	}

	headerJSON, err := b64.DecodeString(parts[0])
	if err != nil {
		return nil, fmt.Errorf("decode header: %w", err)
	}
	payloadJSON, err := b64.DecodeString(parts[1])
	if err != nil {
		return nil, fmt.Errorf("decode payload: %w", err)
	}

	var hdr proofHeader
	if err := json.Unmarshal(headerJSON, &hdr); err != nil {
		return nil, fmt.Errorf("parse header: %w", err)
	}
	if hdr.Typ != typValue {
		return nil, errors.New("unexpected typ header")
	}
	if hdr.Alg != algValue {
		return nil, fmt.Errorf("unexpected alg header %q", hdr.Alg)
	}
	if len(hdr.Jwk) == 0 {
		return nil, errors.New("missing jwk header")
	}

	key, err := parsePublicKey(hdr.Jwk)
	if err != nil {
		return nil, fmt.Errorf("parse jwk: %w", err)
	}

	var claims ProofClaims
	if err := json.Unmarshal(payloadJSON, &claims); err != nil {
		return nil, fmt.Errorf("parse claims: %w", err)
	}

	return &Proof{
		Raw:    raw,
		Header: hdr,
		Claims: claims,
		Key:    key,
	}, nil
}

func parsePublicKey(raw json.RawMessage) (*ecdsa.PublicKey, error) {
	k, err := jwk.ParseKey(raw)
	if err != nil {
		return nil, err
	}

	var pub ecdsa.PublicKey
	if err := jwk.Export(k, &pub); err != nil {
		return nil, err
	}
	if pub.Curve != elliptic.P256() {
		return nil, errors.New("unsupported curve, expected P-256")
	}
	if pub.X == nil || pub.Y == nil || pub.X.Sign() <= 0 || pub.Y.Sign() <= 0 {
		return nil, errors.New("invalid public key point")
	}
	if !pub.Curve.IsOnCurve(pub.X, pub.Y) {
		return nil, errors.New("public key point not on curve")
	}

	return &pub, nil
}

// VerifySignature verifies the JWS signature against the key embedded in the header.
func (p *Proof) VerifySignature() error {
	if _, err := jws.Verify([]byte(p.Raw), jws.WithKey(jwa.ES256(), p.Key)); err != nil {
		return fmt.Errorf("signature verification failed: %w", err)
	}
	return nil
}

// Thumbprint returns the RFC 7638 SHA-256 thumbprint (jkt) of the proof key.
func (p *Proof) Thumbprint() (string, error) {
	return Thumbprint(p.Key)
}

// Thumbprint computes the RFC 7638 SHA-256 thumbprint of an EC P-256 public key.
func Thumbprint(pub *ecdsa.PublicKey) (string, error) {
	if pub.Curve != elliptic.P256() {
		return "", errors.New("unsupported curve, expected P-256")
	}
	size := (pub.Curve.Params().BitSize + 7) / 8

	canonical := fmt.Sprintf(
		`{"crv":"P-256","kty":"EC","x":%q,"y":%q}`,
		b64.EncodeToString(paddedBytes(pub.X.Bytes(), size)),
		b64.EncodeToString(paddedBytes(pub.Y.Bytes(), size)),
	)

	sum := sha256.Sum256([]byte(canonical))
	return b64.EncodeToString(sum[:]), nil
}

// Validate checks the htm, htu, iat and jti claims of the proof against the request.
func (p *Proof) Validate(method string, htu string, now time.Time, maxAge, futureSkew time.Duration) error {
	if p.Claims.Jti == "" {
		return errors.New("missing jti claim")
	}
	if p.Claims.Iat == 0 {
		return errors.New("missing iat claim")
	}

	if !strings.EqualFold(p.Claims.Htm, method) {
		return fmt.Errorf("htm mismatch: %q != %q", p.Claims.Htm, method)
	}
	if p.Claims.Htu == "" || p.Claims.Htu != htu {
		return fmt.Errorf("htu mismatch: %q != %q", p.Claims.Htu, htu)
	}

	iat := time.Unix(p.Claims.Iat, 0)
	if iat.After(now.Add(futureSkew)) {
		return errors.New("iat too far in the future")
	}
	if now.Sub(iat) > maxAge {
		return errors.New("proof expired")
	}

	return nil
}

// RequestHTU reconstructs the client-facing request URI that a DPoP proof's htu
// must match, honoring the forwarded headers set by the gateway.
func RequestHTU(r *http.Request) string {
	proto := r.Header.Get("X-Forwarded-Proto")
	if proto == "" {
		if r.TLS != nil {
			proto = "https"
		} else {
			proto = "http"
		}
	}

	host := r.Header.Get("X-Forwarded-Host")
	if host == "" {
		host = r.Host
	}

	prefix := r.Header.Get("X-Forwarded-Prefix")

	return proto + "://" + host + prefix + r.URL.Path
}

func paddedBytes(b []byte, size int) []byte {
	if len(b) >= size {
		return b[:size]
	}
	out := make([]byte, size)
	copy(out[size-len(b):], b)
	return out
}