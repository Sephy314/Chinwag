package utils

import (
	"crypto/ecdsa"
	"time"

	"github.com/Sephy314/chinwag/auth/domain"
	"github.com/lestrrat-go/jwx/v3/jwa"
	"github.com/lestrrat-go/jwx/v3/jwk"
	"github.com/lestrrat-go/jwx/v3/jws"
	"github.com/lestrrat-go/jwx/v3/jwt"
)

func NewToken(subject string, role domain.Role, key *ecdsa.PrivateKey, kid string) (*string, error) {
	token, err := jwt.NewBuilder().
		Subject(subject).
		Issuer("github.com/Sephy314/Chinwag").
		IssuedAt(time.Now()).
		Claim("role", role).
		Expiration(time.Now().Add(time.Minute * 15)).
		Build()

	if err != nil {
		return nil, err
	}

	headers := jws.NewHeaders()
	err = headers.Set("kid", kid)
	if err != nil {
		return nil, err
	}

	signed, err := jwt.Sign(
		token,
		jwt.WithKey(
			jwa.ES256(),
			key,
			jws.WithProtectedHeaders(headers),
		),
	)

	if err != nil {
		return nil, err
	}

	return new(string(signed)), nil
}

func VerifyToken(token string, keys *jwk.Set) (*jwt.Token, error) {
	parsed, err := jwt.Parse(
		[]byte(token),
		jwt.WithKeySet(*keys),
	)

	if err != nil {
		return nil, err
	}

	return &parsed, nil
}
