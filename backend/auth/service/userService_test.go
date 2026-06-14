package service_test

import (
	"context"
	"errors"
	"testing"

	"github.com/Sephy314/chinwag/auth/domain"
	mocked "github.com/Sephy314/chinwag/auth/mocked"
	"github.com/Sephy314/chinwag/auth/service"
	"github.com/Sephy314/chinwag/auth/structs"
	"github.com/Sephy314/chinwag/conn/cache"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func NewTestUserService() (*service.UserService, *mocked.UserRepo) {
	mckedCache := cache.RedisCache{}

	repos := &mocked.UserRepo{}

	svc := service.NewUserService(&mckedCache, repos)

	return svc, repos
}

func TestUserService_CreateUser(t *testing.T) {
	svc, repos := NewTestUserService()

	repos.On("CreateUser", mock.Anything, mock.Anything).Return(nil)

	// Create User Test
	ctx := context.Background()

	uname := "testUser"
	uemail := "testUserEmail"
	tpw := "testUserPassword"

	req := structs.CreateUserReq{
		Username: uname,
		Email:    uemail,
		Password: tpw,
	}

	usr, err := svc.CreateUser(ctx, req)
	if err != nil {
		t.Fatal(err)
	}

	require.Equal(t, uname, usr.Name)
	require.Equal(t, uemail, usr.Email)
}
func TestUserService_GetUser(t *testing.T) {
	ctx := context.Background()

	mockRepo := new(mocked.UserRepo)

	svc := &service.UserService{
		Repo: mockRepo,
	}

	expected := &domain.User{
		Id:    "123",
		Name:  "davie",
		Email: "davie@test.com",
	}

	mockRepo.
		On("GetUser", ctx, "123").
		Return(expected, nil)

	user, err := svc.GetUser(ctx, "123")

	assert.NoError(t, err)
	assert.Equal(t, expected, user)

	mockRepo.AssertExpectations(t)
}

func TestUserService_DeleteUser(t *testing.T) {
	ctx := context.Background()

	mockRepo := new(mocked.UserRepo)

	svc := &service.UserService{
		Repo: mockRepo,
	}

	mockRepo.
		On("DeleteUser", ctx, "123").
		Return(nil)

	err := svc.DeleteUser(ctx, "123")

	assert.NoError(t, err)

	mockRepo.AssertExpectations(t)
}

func TestUserService_GetUser_Error(t *testing.T) {
	ctx := context.Background()

	mockRepo := new(mocked.UserRepo)

	svc := &service.UserService{
		Repo: mockRepo,
	}

	expectedErr := errors.New("db error")

	mockRepo.
		On("GetUser", ctx, "123").
		Return((*domain.User)(nil), expectedErr)

	user, err := svc.GetUser(ctx, "123")

	assert.Nil(t, user)
	assert.ErrorIs(t, err, expectedErr)
}
