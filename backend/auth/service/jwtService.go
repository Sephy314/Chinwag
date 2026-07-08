package service

import (
	"context"
	"crypto/ecdsa"

	"github.com/Sephy314/chinwag/auth/domain"
	"github.com/Sephy314/chinwag/auth/errs"
	"github.com/Sephy314/chinwag/shared/utils"
	"github.com/golang-jwt/jwt/v5"
	"github.com/lestrrat-go/jwx/v3/jwk"
)

type JwtServiceInterface interface {
	NewAccessToken(ctx context.Context, userId string, role domain.Role) (*string, error)
	KeyFunc(token *jwt.Token) (any, error)
}

type JwtServiceImpl struct {
	refreshService RefreshTokenServiceInterface
	jwksService    JwksServiceInterface
}

func NewJwtService(refreshService RefreshTokenServiceInterface, jwksService JwksServiceInterface) JwtServiceInterface {
	return &JwtServiceImpl{
		refreshService: refreshService,
		jwksService:    jwksService,
	}
}

func (s *JwtServiceImpl) NewAccessToken(ctx context.Context, userId string, role domain.Role) (*string, error) {
	key, err := s.jwksService.GetActiveKey(ctx)

	if err != nil {
		return nil, err
	}

	priv := key.PrivateKey

	return utils.NewToken(userId, role, priv, key.Kid)
}

func (s *JwtServiceImpl) KeyFunc(token *jwt.Token) (any, error) {
	if token.Method.Alg() != jwt.SigningMethodES256.Alg() {
		return nil, errs.InvalidAlgErr
	}

	kid, ok := token.Header["kid"].(string)
	if !ok {
		return nil, errs.InvalidTokenErr
	}

	jwks, err := s.jwksService.GetJwkSet(context.Background())
	if err != nil {
		return nil, err
	}

	key, ok := jwks.LookupKeyID(kid)
	if !ok {
		return nil, errs.InvalidTokenErr
	}

	var publicKey ecdsa.PublicKey

	if err := jwk.Export(key, &publicKey); err != nil {
		return nil, err
	}

	return &publicKey, nil
}
