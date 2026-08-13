package service

import (
	"context"
	"errors"
	"testing"

	"github.com/Sephy314/chinwag/backend/services/auth/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestJwtService_NewAccessToken_Success(t *testing.T) {
	jwks := new(MockJwksService)
	refresh := new(MockRefreshTokenService)
	svc := NewJwtService(refresh, jwks)

	key := makeSigningKey(t)
	jwks.On("GetActiveAccessKey", mock.Anything).Return(key, nil).Once()

	token, err := svc.NewAccessToken(context.Background(), "u1", domain.USER, "jkt")

	assert.NoError(t, err)
	assert.NotNil(t, token)
	assert.NotEmpty(t, *token)
	jwks.AssertExpectations(t)
}

func TestJwtService_NewAccessToken_NoJkt(t *testing.T) {
	jwks := new(MockJwksService)
	refresh := new(MockRefreshTokenService)
	svc := NewJwtService(refresh, jwks)

	key := makeSigningKey(t)
	jwks.On("GetActiveAccessKey", mock.Anything).Return(key, nil).Once()

	token, err := svc.NewAccessToken(context.Background(), "u1", domain.MANAGER, "")

	assert.NoError(t, err)
	assert.NotNil(t, token)
	assert.NotEmpty(t, *token)
	jwks.AssertExpectations(t)
}

func TestJwtService_NewAccessToken_JwksError(t *testing.T) {
	jwks := new(MockJwksService)
	refresh := new(MockRefreshTokenService)
	svc := NewJwtService(refresh, jwks)

	jwks.On("GetActiveAccessKey", mock.Anything).Return(nil, errors.New("no key")).Once()

	token, err := svc.NewAccessToken(context.Background(), "u1", domain.USER, "jkt")

	assert.Nil(t, token)
	assert.EqualError(t, err, "no key")
	jwks.AssertExpectations(t)
}
