package jwt

import (
	"crypto/ecdsa"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/lestrrat-go/jwx/v3/jwa"
	"github.com/lestrrat-go/jwx/v3/jws"
	jwxjwt "github.com/lestrrat-go/jwx/v3/jwt"
)

// Refresh token issuer/audience. RTs are only consumed by the auth service
// itself, but the claims are still pinned so a token minted by another issuer
// (or for another audience) can never be accepted.
const (
	RefreshTokenIssuer   = "chinwag-auth"
	RefreshTokenAudience = "chinwag"
)

// VerifiedRefreshToken carries the claims extracted from a cryptographically
// valid refresh-token JWT.
type VerifiedRefreshToken struct {
	Subject  string
	JTI      string
	SID      string
	Jkt      string
	IssuedAt time.Time
	Expires  time.Time
}

// SignRefreshToken builds and signs a refresh-token JWT.
//
// Claims: iss, sub, aud, iat, exp, jti, sid (refresh-token lineage id) and
// cnf.jkt (DPoP key thumbprint). The DPoP binding is MANDATORY: a refresh token
// without a jkt is refused at signing time so no call path can accidentally
// mint an unbound RT. The token is signed with a Refresh-type key; callers must
// guarantee the key's type is Refresh.
func SignRefreshToken(subject, jti, sid, jkt string, privateKey *ecdsa.PrivateKey, kid string, now time.Time, ttl time.Duration) (string, error) {
	if jkt == "" {
		return "", errors.New("refresh token requires a DPoP jkt binding")
	}

	builder := jwxjwt.NewBuilder().
		Issuer(RefreshTokenIssuer).
		Subject(subject).
		Audience([]string{RefreshTokenAudience}).
		IssuedAt(now).
		Expiration(now.Add(ttl)).
		JwtID(jti).
		Claim("sid", sid).
		Claim("cnf", map[string]string{"jkt": jkt})

	token, err := builder.Build()
	if err != nil {
		return "", err
	}

	headers := jws.NewHeaders()
	if err := headers.Set("kid", kid); err != nil {
		return "", err
	}

	signed, err := jwxjwt.Sign(
		token,
		jwxjwt.WithKey(
			jwa.ES256(),
			privateKey,
			jws.WithProtectedHeaders(headers),
		),
	)
	if err != nil {
		return "", err
	}

	return string(signed), nil
}

// VerifyRefreshToken validates the signature (ES256), the protected header
// algorithm, and the required claims (iss, aud, iat, exp, jti, sid) of a
// refresh-token JWT against the given public key and reference clock. It
// returns the extracted claims along with the token's kid so the caller can
// assert the signing-key type. The alg check is an explicit whitelist to
// prevent algorithm confusion (e.g. HS256/none).
func VerifyRefreshToken(raw string, pub *ecdsa.PublicKey, now time.Time) (*VerifiedRefreshToken, string, error) {
	msg, err := jws.Parse([]byte(raw))
	if err != nil {
		return nil, "", fmt.Errorf("parse refresh token: %w", err)
	}

	sigs := msg.Signatures()
	if len(sigs) != 1 {
		return nil, "", errors.New("refresh token must have exactly one signature")
	}

	hdr := sigs[0].ProtectedHeaders()
	alg, ok := hdr.Algorithm()
	if !ok || alg != jwa.ES256() {
		return nil, "", errors.New("unexpected refresh token alg")
	}
	kid, _ := hdr.KeyID()
	if kid == "" {
		return nil, "", errors.New("refresh token missing kid")
	}

	payload, err := jws.Verify([]byte(raw), jws.WithKey(jwa.ES256(), pub))
	if err != nil {
		return nil, "", fmt.Errorf("invalid refresh token signature: %w", err)
	}

	tok, err := jwxjwt.Parse(
		payload,
		// The signature was already verified above with jws.Verify, so only
		// claim validation is needed here.
		jwxjwt.WithVerify(false),
		jwxjwt.WithValidate(true),
		jwxjwt.WithIssuer(RefreshTokenIssuer),
		jwxjwt.WithAudience(RefreshTokenAudience),
		jwxjwt.WithClock(jwxjwt.ClockFunc(func() time.Time { return now })),
	)
	if err != nil {
		return nil, "", fmt.Errorf("invalid refresh token claims: %w", err)
	}

	jti, ok := tok.JwtID()
	if !ok || jti == "" {
		return nil, "", errors.New("refresh token missing jti")
	}

	var sid string
	if err := tok.Get("sid", &sid); err != nil || sid == "" {
		return nil, "", errors.New("refresh token missing sid")
	}

	subject, ok := tok.Subject()
	if !ok || subject == "" {
		return nil, "", errors.New("refresh token missing sub")
	}

	iat, _ := tok.IssuedAt()
	exp, _ := tok.Expiration()

	// Extract the DPoP confirmation thumbprint directly from the verified
	// payload (jwx stores non-standard claims opaquely, so unmarshal the raw
	// claim JSON rather than round-tripping through Token.Get).
	var rawClaims struct {
		CNF *struct {
			Jkt string `json:"jkt"`
		} `json:"cnf"`
	}
	jkt := ""
	if err := json.Unmarshal(payload, &rawClaims); err == nil && rawClaims.CNF != nil {
		jkt = rawClaims.CNF.Jkt
	}

	return &VerifiedRefreshToken{
		Subject:  subject,
		JTI:      jti,
		SID:      sid,
		Jkt:      jkt,
		IssuedAt: iat,
		Expires:  exp,
	}, kid, nil
}
