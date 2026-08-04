package jwt

import (
	"crypto/ecdsa"
	"time"

	"github.com/lestrrat-go/jwx/v3/jwa"
	"github.com/lestrrat-go/jwx/v3/jws"
	jwxjwt "github.com/lestrrat-go/jwx/v3/jwt"
)

func Sign(subject string, role string, privateKey *ecdsa.PrivateKey, kid string) (string, error) {
	return SignWithCNF(subject, role, privateKey, kid, "")
}

// SignWithCNF signs an access token optionally bound to a DPoP key via the
// cnf.jkt claim (RFC 9449 section 4.1).
func SignWithCNF(subject string, role string, privateKey *ecdsa.PrivateKey, kid, jkt string) (string, error) {
	builder := jwxjwt.NewBuilder().
		Subject(subject).
		Issuer("github.com/Sephy314/Chinwag").
		IssuedAt(time.Now()).
		Claim("role", role).
		Expiration(time.Now().Add(time.Minute * 15))

	if jkt != "" {
		builder = builder.Claim("cnf", map[string]string{"jkt": jkt})
	}

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
