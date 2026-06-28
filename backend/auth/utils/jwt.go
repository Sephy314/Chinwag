package utils

import (
	"crypto/ecdsa"
	"time"

	"github.com/lestrrat-go/jwx/v3/jwa"
	"github.com/lestrrat-go/jwx/v3/jwk"
	"github.com/lestrrat-go/jwx/v3/jwt"
)

func NewToken(subject string, key *ecdsa.PrivateKey) (*string, error) {
	token, err := jwt.NewBuilder().
		Subject(subject).
		Issuer("github.com/Sephy314/Chinwag").
		IssuedAt(time.Now()).
		Expiration(time.Now().Add(time.Minute * 15)).
		Build()

	if err != nil {
		return nil, err
	}

	signed, err := jwt.Sign(
		token,
		jwt.WithKey(jwa.ES256(), key),
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
