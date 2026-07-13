package service

import (
	"context"
	"time"

	"github.com/Sephy314/chinwag/auth/structs"
	"github.com/Sephy314/chinwag/conn/cache"
	"github.com/Sephy314/chinwag/shared/errs"
)

type RefreshTokenServiceInterface interface {
	GetUserIdByRefreshToken(ctx context.Context, refreshToken string) (*string, error)
	InsertRefreshToken(ctx context.Context, token structs.RefreshToken) error
	RemoveRefreshToken(ctx context.Context, tokenId string) error
}

type RefreshTokenService struct {
	Cache              cache.Cache
	RefreshTokenPrefix string
	RefreshTokenTTL    time.Duration
}

func (r *RefreshTokenService) GetUserIdByRefreshToken(ctx context.Context, refreshToken string) (*string, error) {
	key := r.RefreshTokenPrefix + refreshToken

	token, err := r.Cache.Get(ctx, key)
	if err != nil {
		return nil, errs.ErrCacheNotFound
	}

	return &token, nil
}

func (r *RefreshTokenService) InsertRefreshToken(ctx context.Context, token structs.RefreshToken) error {
	return r.Cache.Set(
		ctx,
		r.RefreshTokenPrefix+token.RefreshToken,
		token.Subject,
		time.Hour*24*14,
	)
}

func (r *RefreshTokenService) RemoveRefreshToken(ctx context.Context, tokenId string) error {
	return r.Cache.Delete(
		ctx,
		r.RefreshTokenPrefix+tokenId,
	)
}

func NewRefreshTokenService(cache cache.Cache, prefix string, ttl time.Duration) *RefreshTokenService {
	return &RefreshTokenService{
		Cache:              cache,
		RefreshTokenPrefix: prefix,
		RefreshTokenTTL:    ttl,
	}
}
