package keyProvider

import (
	"context"
	"crypto/ecdsa"
)

type KeyProvider interface {
	GetPublicKey(ctx context.Context, kid string) (*ecdsa.PublicKey, error)
}

var provider KeyProvider

func InjectProvider(p KeyProvider) {
	provider = p
}
