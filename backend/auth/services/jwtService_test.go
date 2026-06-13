package services_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Sephy314/chinwag/auth/domain"
	"github.com/Sephy314/chinwag/auth/mock"
	"github.com/Sephy314/chinwag/auth/services"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestJwksService_Rotate(t *testing.T) {
	ctx := context.Background()

	mockRepo := new(mocked.JwkRepo)

	mockRepo.
		On(
			"Rotate",
			ctx,
			mock.MatchedBy(func(key domain.SigningKeyEntity) bool {
				return key.Status == domain.Active &&
					key.PublicKey != "" &&
					key.PrivateKey != "" &&
					key.Kid != ""
			}),
		).
		Return(nil)

	svc := services.NewJwksService(mockRepo)

	err := svc.Rotate(ctx)

	assert.NoError(t, err)
	mockRepo.AssertExpectations(t)
}

func TestJwksService_LoadJWKS_NoReload(t *testing.T) {
	ctx := context.Background()

	mockRepo := new(mocked.JwkRepo)

	mockRepo.
		On("GetVersion", ctx).
		Return(new(time.Now()), nil)

	mockRepo.
		On("Load", ctx).
		Return([]domain.SigningKeyEntity{}, nil)

	mockRepo.
		On("Count", mock.Anything).
		Return(new(int64(1)), nil)

	svc := services.NewJwksService(mockRepo)

	err := svc.LoadJWKS(ctx)

	assert.NoError(t, err)

	mockRepo.AssertNotCalled(t, "Load")
	mockRepo.AssertExpectations(t)
}

func TestJwksService_LoadJWKS_GetVersionError(t *testing.T) {
	ctx := context.Background()

	mockRepo := new(mocked.JwkRepo)

	mockRepo.
		On("GetVersion", ctx).
		Return((*time.Time)(nil), errors.New("db error"))

	mockRepo.
		On("Count", mock.Anything).
		Return(new(int64(1)), nil)

	svc := services.NewJwksService(mockRepo)

	err := svc.LoadJWKS(ctx)

	assert.Error(t, err)
	mockRepo.AssertExpectations(t)
}
