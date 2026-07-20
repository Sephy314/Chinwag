package service_test

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	crand "crypto/rand"
	"errors"
	"testing"

	"golang.org/x/crypto/bcrypt"

	"github.com/Sephy314/chinwag/auth/domain"
	"github.com/Sephy314/chinwag/auth/mocked"
	"github.com/Sephy314/chinwag/auth/service"
	"github.com/Sephy314/chinwag/auth/structs"
	"github.com/Sephy314/chinwag/conn/cache"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func NewTestUserService() (*service.UserService, *mocked.UserRepo) {
	mockedCache := cache.RedisCache{}
	mockedJwkService := mocked.JwkService{}
	mockedRefreshService := mocked.RefreshTokenService{}

	repos := &mocked.UserRepo{}

	svc := service.NewUserService(&mockedCache, repos, &mockedJwkService, &mockedRefreshService, nil)

	return svc, repos
}

func TestUserService_CreateUser(t *testing.T) {
	svc, repos := NewTestUserService()

	repos.On("CreateUser", mock.Anything, mock.Anything).Return(nil)

	// Create User Test
	ctx := context.Background()

	uname := "testUser"
	userEmail := "testUserEmail"

	tpw := "testUserPassword"

	req := structs.CreateUserReq{
		Username: uname,
		Email:    userEmail,
		Password: tpw,
	}

	usr, err := svc.CreateUser(ctx, req)
	if err != nil {
		t.Fatal(err)
	}

	require.Equal(t, uname, usr.Name)
	require.Equal(t, userEmail, usr.Email)
}

func TestUserService_GetUser(t *testing.T) {
	ctx := context.Background()

	mockRepo := new(mocked.UserRepo)

	svc := &service.UserService{
		Repo: mockRepo,
	}

	expected := &domain.User{
		Id:    "123",
		Name:  "Davie",
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

// Factories/helpers
func newMocks() (*mocked.UserRepo, *mocked.JwkService, *mocked.RefreshTokenService) {
	return new(mocked.UserRepo), new(mocked.JwkService), new(mocked.RefreshTokenService)
}

func newUserWithPassword(t *testing.T, pw string, email string) *domain.User {
	if pw == "" {
		pw = "testPassword"
	}
	if email == "" {
		email = "tester@example.com"
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(pw), bcrypt.MinCost)
	require.NoError(t, err)

	return &domain.User{
		Id:       "uid-1",
		Name:     "tester",
		Email:    email,
		Password: string(hash),
	}
}

func newSigningKey(t *testing.T) *domain.SigningKey {
	priv, err := ecdsa.GenerateKey(elliptic.P256(), crand.Reader)
	require.NoError(t, err)

	return &domain.SigningKey{
		Kid:        "kid",
		PublicKey:  &priv.PublicKey,
		PrivateKey: priv,
	}
}

func newService(repo *mocked.UserRepo, jwk *mocked.JwkService, refresh *mocked.RefreshTokenService) *service.UserService {
	return &service.UserService{
		Repo:           repo,
		JwkService:     jwk,
		RefreshService: refresh,
	}
}

func TestUserService_Login(t *testing.T) {
	ctx := context.Background()

	mockRepo, mockJwk, mockRefresh := newMocks()

	user := newUserWithPassword(t, "testPassword", "tester@example.com")

	mockRepo.On("GetUserByEmail", mock.Anything, user.Email).Return(user, nil)

	sk := newSigningKey(t)
	mockJwk.On("GetActiveKey", mock.Anything).Return(sk, nil)

	mockRefresh.On("InsertRefreshToken", mock.Anything, mock.Anything).Return(nil)

	svc := newService(mockRepo, mockJwk, mockRefresh)

	tokens, err := svc.Login(ctx, user.Email, "testPassword")
	require.NoError(t, err)
	require.NotNil(t, tokens)
	require.NotEmpty(t, tokens.AccessToken)
	require.NotEmpty(t, tokens.RefreshToken)

	mockRepo.AssertExpectations(t)
	mockJwk.AssertExpectations(t)
	mockRefresh.AssertExpectations(t)
}

func TestUserService_Login_WrongPassword(t *testing.T) {
	ctx := context.Background()

	mockRepo, mockJwk, mockRefresh := newMocks()

	user := newUserWithPassword(t, "testPassword", "tester@example.com")

	mockRepo.On("GetUserByEmail", mock.Anything, user.Email).Return(user, nil)

	svc := newService(mockRepo, mockJwk, mockRefresh)

	// Wrong password
	tokens, err := svc.Login(ctx, user.Email, "wrong-password")
	assert.Error(t, err)
	assert.Nil(t, tokens)

	// only repo had expectations
	mockRepo.AssertExpectations(t)
}

func TestUserService_Login_JwkError(t *testing.T) {
	ctx := context.Background()

	mockRepo, mockJwk, _ := newMocks()

	user := newUserWithPassword(t, "testPassword", "tester@example.com")

	mockRepo.On("GetUserByEmail", mock.Anything, user.Email).Return(user, nil)

	mockJwk.On("GetActiveKey", mock.Anything).Return((*domain.SigningKey)(nil), errors.New("jwk fetch failed"))

	svc := newService(mockRepo, mockJwk, nil)

	tokens, err := svc.Login(ctx, user.Email, "testPassword")
	assert.Error(t, err)
	assert.Nil(t, tokens)

	mockRepo.AssertExpectations(t)
	mockJwk.AssertExpectations(t)
}

func TestUserService_Login_RefreshInsertError(t *testing.T) {
	ctx := context.Background()

	mockRepo, mockJwk, mockRefresh := newMocks()

	user := newUserWithPassword(t, "testPassword", "tester@example.com")

	mockRepo.On("GetUserByEmail", mock.Anything, user.Email).Return(user, nil)

	sk := newSigningKey(t)
	mockJwk.On("GetActiveKey", mock.Anything).Return(sk, nil)

	mockRefresh.On("InsertRefreshToken", mock.Anything, mock.Anything).Return(errors.New("db insert failed"))

	svc := newService(mockRepo, mockJwk, mockRefresh)

	tokens, err := svc.Login(ctx, user.Email, "testPassword")
	assert.Error(t, err)
	assert.Nil(t, tokens)

	mockRepo.AssertExpectations(t)
	mockJwk.AssertExpectations(t)
	mockRefresh.AssertExpectations(t)
}

func TestUserService_Login_GetUser_Error_SQLi(t *testing.T) {
	ctx := context.Background()

	mockRepo, mockJwk, _ := newMocks()

	maliciousEmail := "test' OR '1'='1"

	mockRepo.On("GetUserByEmail", mock.Anything, maliciousEmail).Return((*domain.User)(nil), errors.New("sql error"))

	svc := newService(mockRepo, mockJwk, nil)

	tokens, err := svc.Login(ctx, maliciousEmail, "doesn't matter")
	assert.Error(t, err)
	assert.Nil(t, tokens)

	mockRepo.AssertExpectations(t)
}

func TestUserService_UpdateUser_Success(t *testing.T) {
	ctx := context.Background()

	mockRepo := new(mocked.UserRepo)
	svc := &service.UserService{Repo: mockRepo}

	existing := &domain.User{
		Id:    "uid-1",
		Name:  "oldName",
		Email: "old@example.com",
	}

	newName := "newName"
	req := structs.UpdateUserReq{Name: &newName}

	mockRepo.On("GetUser", ctx, "uid-1").Return(existing, nil)
	mockRepo.On("UpdateUser", ctx, mock.AnythingOfType("domain.User")).Return(nil)

	user, err := svc.UpdateUser(ctx, "uid-1", req)

	assert.NoError(t, err)
	assert.Equal(t, "newName", user.Name)
	assert.Equal(t, "old@example.com", user.Email)
	mockRepo.AssertExpectations(t)
}

func TestUserService_UpdateUser_NotFound(t *testing.T) {
	ctx := context.Background()

	mockRepo := new(mocked.UserRepo)
	svc := &service.UserService{Repo: mockRepo}

	newName := "newName"
	req := structs.UpdateUserReq{Name: &newName}

	mockRepo.On("GetUser", ctx, "uid-nonexistent").Return((*domain.User)(nil), errors.New("sql: no rows in result set"))

	user, err := svc.UpdateUser(ctx, "uid-nonexistent", req)

	assert.Error(t, err)
	assert.Nil(t, user)
	mockRepo.AssertExpectations(t)
}

func TestUserService_UpdateUser_PasswordHashed(t *testing.T) {
	ctx := context.Background()

	mockRepo := new(mocked.UserRepo)
	svc := &service.UserService{Repo: mockRepo}

	existing := &domain.User{
		Id:       "uid-1",
		Name:     "tester",
		Email:    "tester@example.com",
		Password: "oldHash",
	}

	newPw := "newPassword123"
	req := structs.UpdateUserReq{Password: &newPw}

	mockRepo.On("GetUser", ctx, "uid-1").Return(existing, nil)
	mockRepo.On("UpdateUser", ctx, mock.AnythingOfType("domain.User")).Return(nil)

	user, err := svc.UpdateUser(ctx, "uid-1", req)

	assert.NoError(t, err)
	assert.NotEqual(t, "newPassword123", user.Password)
	assert.NotEqual(t, "oldHash", user.Password)

	err = bcrypt.CompareHashAndPassword([]byte(user.Password), []byte("newPassword123"))
	assert.NoError(t, err)

	mockRepo.AssertExpectations(t)
}

func TestUserService_UpdateUser_DBError(t *testing.T) {
	ctx := context.Background()

	mockRepo := new(mocked.UserRepo)
	svc := &service.UserService{Repo: mockRepo}

	existing := &domain.User{
		Id:    "uid-1",
		Name:  "tester",
		Email: "tester@example.com",
	}

	newName := "updated"
	req := structs.UpdateUserReq{Name: &newName}

	mockRepo.On("GetUser", ctx, "uid-1").Return(existing, nil)
	mockRepo.On("UpdateUser", ctx, mock.AnythingOfType("domain.User")).Return(errors.New("db connection lost"))

	user, err := svc.UpdateUser(ctx, "uid-1", req)

	assert.Error(t, err)
	assert.Nil(t, user)
	assert.Contains(t, err.Error(), "db connection lost")
	mockRepo.AssertExpectations(t)
}
