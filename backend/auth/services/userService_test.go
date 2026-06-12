package services_test

import (
	"context"
	"testing"

	mocked "github.com/Sephy314/chinwag/auth/mock"
	"github.com/Sephy314/chinwag/auth/services"
	"github.com/Sephy314/chinwag/auth/structs"
	"github.com/Sephy314/chinwag/conn/cache"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func NewTestUserService() (*services.UserService, *mocked.UserRepo) {
	mckedCache := cache.RedisCache{}

	repos := &mocked.UserRepo{}

	svc := services.NewUserService(&mckedCache, repos)

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
