package jwt

import (
	"crypto/ecdsa"
	"crypto/x509"
	"encoding/base64"
	"errors"

	"github.com/Sephy314/chinwag/backend/services/auth/domain"
)

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
