package jwt

import (
	"github.com/Sephy314/chinwag/backend/services/auth/domain"
	"github.com/lestrrat-go/jwx/v3/jwk"
)

// ToJWKSet builds the PUBLIC JWKS served to token verifiers (gateway, frontend,
// other services). Only Access-token signing keys are published — Refresh keys
// are verified internally by the auth service and are never exposed to clients.
func ToJWKSet(keys []domain.SigningKeyEntity) (jwk.Set, error) {
	set := jwk.NewSet()

	for _, k := range keys {
		if k.Type != domain.KeyTypeAccess {
			continue
		}
		if !k.Status.IncludeInJWKS() {
			continue
		}

		pb, err := DecodePublicKey(k.PublicKey)
		if err != nil {
			return nil, err
		}

		jwkKey, err := jwk.Import(pb)
		if err != nil {
			return nil, err
		}

		_ = jwkKey.Set(jwk.KeyIDKey, k.Kid)
		_ = jwkKey.Set(jwk.KeyUsageKey, "sig")
		_ = jwkKey.Set(jwk.AlgorithmKey, "ES256")

		if err := set.AddKey(jwkKey); err != nil {
			return nil, err
		}
	}

	return set, nil
}
