package service_test

import (
	"context"
	"testing"
	"time"

	"github.com/Sephy314/chinwag/auth/domain"
	"github.com/Sephy314/chinwag/auth/mocked"
	"github.com/Sephy314/chinwag/auth/service"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestJwksService_Rotate(t *testing.T) {
	ctx := context.Background()

	mockRepo := new(mocked.JwkRepo)

	tm := time.Now()

	mockRepo.
		On("Rotate",
			mock.Anything,
			mock.MatchedBy(func(key domain.SigningKeyEntity) bool {
				return key.Status == domain.Active &&
					key.PublicKey != "" &&
					key.PrivateKey != "" &&
					key.Kid != ""
			}),
		).
		Return(nil)

	mockRepo.
		On("GetVersion", mock.Anything).
		Return(&tm, nil)

	mockRepo.
		On("Load", mock.Anything).
		Return([]domain.SigningKeyEntity{}, nil)

	svc := service.NewJwksService(mockRepo)

	err := svc.Rotate(ctx)

	require.NoError(t, err)

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

	svc := service.NewJwksService(mockRepo)

	err := svc.LoadJWKS(ctx)

	assert.NoError(t, err)

	mockRepo.AssertNotCalled(t, "Load")
	mockRepo.AssertExpectations(t)
}
