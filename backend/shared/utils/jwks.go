package utils

import (
	"crypto/ecdsa"
	"crypto/x509"
	"encoding/base64"
	"errors"
	"time"

	"github.com/Sephy314/chinwag/auth/domain"
	"github.com/lestrrat-go/jwx/v3/jwk"
)

var Expired = time.Hour * 24

func ToJWKS(keys []domain.SigningKeyEntity) (jwk.Set, error) {
	set := jwk.NewSet()

	for _, k := range keys {
		if !k.Status.IncludeInJWKS() {
			continue
		}

		//if k.PublicKey == nil {
		//	continue
		//}

		pb, err := DecodePublicKey(k.PublicKey)

		jwkKey, err := jwk.Import(pb)
		if err != nil {
			return nil, err
		}

		_ = jwkKey.Set(jwk.KeyIDKey, k.Kid)
		_ = jwkKey.Set(jwk.KeyUsageKey, "sig")
		_ = jwkKey.Set(jwk.AlgorithmKey, "ES256")

		err = set.AddKey(jwkKey)
		if err != nil {
			return nil, err
		}
	}

	return set, nil
}

func GetExpiredAt(t time.Time) time.Time {
	return t.Add(Expired)
}

func EncodePublicKey(pub *ecdsa.PublicKey) (string, error) {
	b, err := x509.MarshalPKIXPublicKey(pub)
	if err != nil {
		return "", err
	}

	return base64.StdEncoding.EncodeToString(b), nil
}

func EncodePrivateKey(priv *ecdsa.PrivateKey) (string, error) {
	b, err := x509.MarshalECPrivateKey(priv)
	if err != nil {
		return "", err
	}

	return base64.StdEncoding.EncodeToString(b), nil
}

func DecodePublicKey(raw string) (*ecdsa.PublicKey, error) {
	b, err := base64.StdEncoding.DecodeString(raw)
	if err != nil {
		return nil, err
	}

	key, err := x509.ParsePKIXPublicKey(b)
	if err != nil {
		return nil, err
	}

	pub, ok := key.(*ecdsa.PublicKey)
	if !ok {
		return nil, errors.New("not ecdsa public key")
	}

	return pub, nil
}

func DecodePrivateKey(raw string) (*ecdsa.PrivateKey, error) {
	b, err := base64.StdEncoding.DecodeString(raw)
	if err != nil {
		return nil, err
	}

	return x509.ParseECPrivateKey(b)
}

func SigningKeyEntityToSigningKey(key domain.SigningKeyEntity) (*domain.SigningKey, error) {
	pub, err := DecodePublicKey(key.PublicKey)
	if err != nil {
		return nil, err
	}

	priv, err := DecodePrivateKey(key.PrivateKey)
	if err != nil {
		return nil, err
	}

	return &domain.SigningKey{
		Kid:        key.Kid,
		PublicKey:  pub,
		PrivateKey: priv,
		Status:     key.Status,
		CreatedAt:  key.CreatedAt,
		UpdatedAt:  key.UpdatedAt,
		ExpiredAt:  key.ExpiredAt,
	}, nil
}
