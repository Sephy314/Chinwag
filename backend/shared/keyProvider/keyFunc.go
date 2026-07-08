package keyProvider

import (
	"context"

	"github.com/Sephy314/chinwag/auth/errs"
	"github.com/golang-jwt/jwt/v5"
)

func KeyFunc(token *jwt.Token) (any, error) {
	if token.Method.Alg() != jwt.SigningMethodES256.Alg() {
		return nil, errs.InvalidAlgErr
	}

	kid, ok := token.Header["kid"].(string)
	if !ok {
		return nil, errs.InvalidTokenErr
	}

	key, err := provider.GetPublicKey(context.Background(), kid)
	if err != nil {
		return nil, err
	}

	return key, nil
}
