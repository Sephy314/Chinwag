package mocked

import (
	"context"

	"github.com/Sephy314/chinwag/auth/structs"
	"github.com/stretchr/testify/mock"
)

type RefreshTokenService struct {
	mock.Mock
}

func (m *RefreshTokenService) GetUserIdByRefreshToken(
	ctx context.Context,
	refreshToken string,
) (*string, error) {
	args := m.Called(ctx, refreshToken)

	var userId *string

	if args.Get(0) != nil {
		userId = args.Get(0).(*string)
	}

	return userId, args.Error(1)
}

func (m *RefreshTokenService) InsertRefreshToken(
	ctx context.Context,
	token structs.RefreshToken,
) error {
	args := m.Called(ctx, token)

	return args.Error(0)
}

func (m *RefreshTokenService) RemoveRefreshToken(
	ctx context.Context,
	tokenId string,
) error {
	args := m.Called(ctx, tokenId)

	return args.Error(0)
}
