package jwt

import (
	"crypto/ecdsa"
	"time"

	"github.com/lestrrat-go/jwx/v3/jwa"
	"github.com/lestrrat-go/jwx/v3/jws"
	jwxjwt "github.com/lestrrat-go/jwx/v3/jwt"
)

func Sign(subject string, role string, privateKey *ecdsa.PrivateKey, kid string) (string, error) {
	token, err := jwxjwt.NewBuilder().
		Subject(subject).
		Issuer("github.com/Sephy314/Chinwag").
		IssuedAt(time.Now()).
		Claim("role", role).
		Expiration(time.Now().Add(time.Minute * 15)).
		Build()
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
