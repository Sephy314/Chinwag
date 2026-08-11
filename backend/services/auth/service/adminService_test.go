package service

import (
	"context"
	"testing"

	"github.com/Sephy314/chinwag/backend/services/auth/domain"
	"github.com/Sephy314/chinwag/backend/services/auth/shared/errs"
	"github.com/Sephy314/chinwag/backend/services/auth/structs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestUserService_AdminCreateUser_InvalidRole(t *testing.T) {
	userRepo := new(MockUserRepo)
	svc := baseUserService(userRepo, new(MockJwksService), new(MockRefreshTokenService))

	_, err := svc.AdminCreateUser(context.Background(), structs.CreateAdminUserRequest{
		Name: "Bob", Email: "b@example.com", Password: "secret", Role: "SUPERUSER",
	})

	assert.ErrorIs(t, err, errs.ErrInvalidRole)
	userRepo.AssertNotCalled(t, "CreateUser", mock.Anything, mock.Anything)
}

func TestUserService_AdminSetRole_InvalidRole(t *testing.T) {
	userRepo := new(MockUserRepo)
	svc := baseUserService(userRepo, new(MockJwksService), new(MockRefreshTokenService))

	err := svc.AdminSetRole(context.Background(), "u1", "admin1", "ROOT")

	assert.ErrorIs(t, err, errs.ErrInvalidRole)
	userRepo.AssertNotCalled(t, "SetRole", mock.Anything, mock.Anything, mock.Anything)
}

func TestUserService_AdminSetRole_SelfDemotion(t *testing.T) {
	userRepo := new(MockUserRepo)
	svc := baseUserService(userRepo, new(MockJwksService), new(MockRefreshTokenService))

	err := svc.AdminSetRole(context.Background(), "admin1", "admin1", "USER")

	assert.ErrorIs(t, err, errs.ErrSelfDemotion)
	userRepo.AssertNotCalled(t, "SetRole", mock.Anything, mock.Anything, mock.Anything)
}

func TestUserService_AdminSetRole_LastAdmin(t *testing.T) {
	userRepo := new(MockUserRepo)
	svc := baseUserService(userRepo, new(MockJwksService), new(MockRefreshTokenService))

	userRepo.On("GetUserIncludingDeleted", mock.Anything, "admin2").Return(
		&domain.User{Id: "admin2", Role: domain.ADMIN}, nil).Once()
	userRepo.On("CountAdmins", mock.Anything).Return(1, nil).Once()

	err := svc.AdminSetRole(context.Background(), "admin2", "admin1", "USER")

	assert.ErrorIs(t, err, errs.ErrLastAdmin)
	userRepo.AssertNotCalled(t, "SetRole", mock.Anything, mock.Anything, mock.Anything)
}

func TestUserService_AdminSetRole_Success(t *testing.T) {
	userRepo := new(MockUserRepo)
	svc := baseUserService(userRepo, new(MockJwksService), new(MockRefreshTokenService))

	userRepo.On("GetUserIncludingDeleted", mock.Anything, "u1").Return(
		&domain.User{Id: "u1", Role: domain.USER}, nil).Once()
	userRepo.On("SetRole", mock.Anything, "u1", domain.MANAGER).Return(nil).Once()

	err := svc.AdminSetRole(context.Background(), "u1", "admin1", "MANAGER")

	assert.NoError(t, err)
	userRepo.AssertExpectations(t)
}

func TestUserService_AdminDisableUser_LastAdmin(t *testing.T) {
	userRepo := new(MockUserRepo)
	svc := baseUserService(userRepo, new(MockJwksService), new(MockRefreshTokenService))

	userRepo.On("GetUserIncludingDeleted", mock.Anything, "admin2").Return(
		&domain.User{Id: "admin2", Role: domain.ADMIN}, nil).Once()
	userRepo.On("CountAdmins", mock.Anything).Return(1, nil).Once()

	err := svc.AdminDisableUser(context.Background(), "admin2")

	assert.ErrorIs(t, err, errs.ErrLastAdmin)
	userRepo.AssertNotCalled(t, "DeleteUser", mock.Anything, mock.Anything)
}

func TestUserService_AdminDisableUser_Success(t *testing.T) {
	userRepo := new(MockUserRepo)
	svc := baseUserService(userRepo, new(MockJwksService), new(MockRefreshTokenService))

	userRepo.On("GetUserIncludingDeleted", mock.Anything, "u1").Return(
		&domain.User{Id: "u1", Role: domain.USER}, nil).Once()
	userRepo.On("DeleteUser", mock.Anything, "u1").Return(nil).Once()

	err := svc.AdminDisableUser(context.Background(), "u1")

	assert.NoError(t, err)
	userRepo.AssertExpectations(t)
}
