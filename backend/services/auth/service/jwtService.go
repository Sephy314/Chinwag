package service

import (
	"context"

	"github.com/Sephy314/chinwag/backend/services/auth/domain"
	"github.com/Sephy314/chinwag/backend/services/auth/internal/jwt"
)

type JwtServiceInterface interface {
	NewAccessToken(ctx context.Context, userId string, role domain.Role, jkt string) (*string, error)
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

func (s *JwtServiceImpl) NewAccessToken(ctx context.Context, userId string, role domain.Role, jkt string) (*string, error) {
	key, err := s.jwksService.GetActiveAccessKey(ctx)
	if err != nil {
		return nil, err
	}

	token, err := jwt.SignWithCNF(userId, string(role), key.PrivateKey, key.Kid, jkt)
	if err != nil {
		return nil, err
	}

	return &token, nil
}
