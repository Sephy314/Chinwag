package service

import (
	"context"

	"github.com/Sephy314/chinwag/auth/domain"
	"github.com/Sephy314/chinwag/shared/utils"
)

type JwtServiceInterface interface {
	NewAccessToken(ctx context.Context, userId string, role domain.Role) (*string, error)
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
