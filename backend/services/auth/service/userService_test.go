package service

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"errors"
	"testing"

	"github.com/Sephy314/chinwag/backend/services/auth/domain"
	"github.com/Sephy314/chinwag/backend/services/auth/shared/errs"
	"github.com/Sephy314/chinwag/backend/services/auth/structs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func makeSigningKey(t *testing.T) *domain.SigningKey {
	t.Helper()
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	return &domain.SigningKey{
		Kid:        "test-kid",
		PublicKey:  &priv.PublicKey,
		PrivateKey: priv,
		Status:     domain.Active,
	}
}

func baseUserService(userRepo *MockUserRepo, jwk *MockJwksService, refresh *MockRefreshTokenService) *UserService {
	return NewUserService(userRepo, jwk, refresh, &MockLogger{})
}

func TestUserService_CreateUser_Success_NoUow(t *testing.T) {
	userRepo := new(MockUserRepo)
	jwk := new(MockJwksService)
	refresh := new(MockRefreshTokenService)
	svc := baseUserService(userRepo, jwk, refresh)

	req := structs.CreateUserReq{Name: "Alice", Email: "alice@example.com", Password: "secret"}

	userRepo.On("CreateUser", mock.Anything, mock.MatchedBy(func(u domain.User) bool {
		return u.Name == "Alice" && u.Email == "alice@example.com" && u.Password != "" && u.Password != "secret" && u.Id != ""
	})).Return(nil).Once()

	user, err := svc.CreateUser(context.Background(), req)

	assert.NoError(t, err)
	assert.NotNil(t, user)
	assert.Equal(t, "Alice", user.Name)
	assert.NotEqual(t, "secret", user.Password) // must be bcrypt-hashed
	userRepo.AssertExpectations(t)
}

func TestUserService_CreateUser_Success_WithUow(t *testing.T) {
	userRepo := new(MockUserRepo)
	jwtRepo := new(MockJwksRepo)
	jwk := new(MockJwksService)
	refresh := new(MockRefreshTokenService)
	svc := NewUserService(userRepo, jwk, refresh, &MockLogger{}, makeUow(userRepo, jwtRepo))

	req := structs.CreateUserReq{Name: "Alice", Email: "alice@example.com", Password: "secret"}

	userRepo.On("CreateUser", mock.Anything, mock.Anything).Return(nil).Once()

	user, err := svc.CreateUser(context.Background(), req)

	assert.NoError(t, err)
	assert.NotNil(t, user)
	userRepo.AssertExpectations(t)
}

func TestUserService_CreateUser_RepoError(t *testing.T) {
	userRepo := new(MockUserRepo)
	jwk := new(MockJwksService)
	refresh := new(MockRefreshTokenService)
	svc := baseUserService(userRepo, jwk, refresh)

	req := structs.CreateUserReq{Name: "Alice", Email: "alice@example.com", Password: "secret"}
	userRepo.On("CreateUser", mock.Anything, mock.Anything).Return(errs.ErrConflict).Once()

	user, err := svc.CreateUser(context.Background(), req)

	assert.Nil(t, user)
	assert.ErrorIs(t, err, errs.ErrConflict)
	userRepo.AssertExpectations(t)
}

func TestUserService_GetUser_Success(t *testing.T) {
	userRepo := new(MockUserRepo)
	jwk := new(MockJwksService)
	refresh := new(MockRefreshTokenService)
	svc := baseUserService(userRepo, jwk, refresh)

	id := "user-1"
	expected := &domain.User{Id: id, Name: "Alice", Email: "alice@example.com"}

	userRepo.On("GetUser", mock.Anything, id).Return(expected, nil).Once()

	user, err := svc.GetUser(context.Background(), id)

	assert.NoError(t, err)
	assert.Equal(t, "Alice", user.Name)
	userRepo.AssertExpectations(t)
}

func TestUserService_GetUser_NotFound(t *testing.T) {
	userRepo := new(MockUserRepo)
	jwk := new(MockJwksService)
	refresh := new(MockRefreshTokenService)
	svc := baseUserService(userRepo, jwk, refresh)

	userRepo.On("GetUser", mock.Anything, "nope").Return(nil, errs.ErrNotFound).Once()

	user, err := svc.GetUser(context.Background(), "nope")

	assert.Nil(t, user)
	assert.ErrorIs(t, err, errs.ErrNotFound)
	userRepo.AssertExpectations(t)
}

func TestUserService_GetUserByEmail_Success(t *testing.T) {
	userRepo := new(MockUserRepo)
	jwk := new(MockJwksService)
	refresh := new(MockRefreshTokenService)
	svc := baseUserService(userRepo, jwk, refresh)

	email := "alice@example.com"
	expected := &domain.User{Id: "u1", Email: email}

	userRepo.On("GetUserByEmail", mock.Anything, email).Return(expected, nil).Once()

	user, err := svc.GetUserByEmail(context.Background(), email)

	assert.NoError(t, err)
	assert.NotNil(t, user)
	userRepo.AssertExpectations(t)
}

func TestUserService_DeleteUser_Success_NoUow(t *testing.T) {
	userRepo := new(MockUserRepo)
	jwk := new(MockJwksService)
	refresh := new(MockRefreshTokenService)
	svc := baseUserService(userRepo, jwk, refresh)

	userRepo.On("DeleteUser", mock.Anything, "u1").Return(nil).Once()

	err := svc.DeleteUser(context.Background(), "u1")

	assert.NoError(t, err)
	userRepo.AssertExpectations(t)
}

func TestUserService_DeleteUser_Success_WithUow(t *testing.T) {
	userRepo := new(MockUserRepo)
	jwtRepo := new(MockJwksRepo)
	jwk := new(MockJwksService)
	refresh := new(MockRefreshTokenService)
	svc := NewUserService(userRepo, jwk, refresh, &MockLogger{}, makeUow(userRepo, jwtRepo))

	userRepo.On("DeleteUser", mock.Anything, "u1").Return(nil).Once()

	err := svc.DeleteUser(context.Background(), "u1")

	assert.NoError(t, err)
	userRepo.AssertExpectations(t)
}

func TestUserService_DeleteUser_Error(t *testing.T) {
	userRepo := new(MockUserRepo)
	jwk := new(MockJwksService)
	refresh := new(MockRefreshTokenService)
	svc := baseUserService(userRepo, jwk, refresh)

	userRepo.On("DeleteUser", mock.Anything, "u1").Return(errors.New("db down")).Once()

	err := svc.DeleteUser(context.Background(), "u1")

	assert.EqualError(t, err, "db down")
	userRepo.AssertExpectations(t)
}

func TestUserService_UpdateUser_Success_NoUow(t *testing.T) {
	userRepo := new(MockUserRepo)
	jwk := new(MockJwksService)
	refresh := new(MockRefreshTokenService)
	svc := baseUserService(userRepo, jwk, refresh)

	id := "u1"
	existing := &domain.User{Id: id, Name: "Old", Email: "old@example.com"}
	newName := "NewName"
	newEmail := "new@example.com"

	userRepo.On("GetUser", mock.Anything, id).Return(existing, nil).Once()
	userRepo.On("UpdateUser", mock.Anything, mock.MatchedBy(func(u domain.User) bool {
		return u.Name == "NewName" && u.Email == "new@example.com"
	})).Return(nil).Once()

	user, err := svc.UpdateUser(context.Background(), id, structs.UpdateUserReq{Name: &newName, Email: &newEmail})

	assert.NoError(t, err)
	assert.Equal(t, "NewName", user.Name)
	assert.Equal(t, "new@example.com", user.Email)
	userRepo.AssertExpectations(t)
}

func TestUserService_UpdateUser_PasswordHashed(t *testing.T) {
	userRepo := new(MockUserRepo)
	jwk := new(MockJwksService)
	refresh := new(MockRefreshTokenService)
	svc := baseUserService(userRepo, jwk, refresh)

	id := "u1"
	existing := &domain.User{Id: id, Name: "Alice", Email: "a@example.com"}
	newPw := "newsecret"
	originalPw := existing.Password // "" — capture before the service mutates the struct

	userRepo.On("GetUser", mock.Anything, id).Return(existing, nil).Once()
	userRepo.On("UpdateUser", mock.Anything, mock.MatchedBy(func(u domain.User) bool {
		return u.Password != "newsecret" && u.Password != originalPw
	})).Return(nil).Once()

	user, err := svc.UpdateUser(context.Background(), id, structs.UpdateUserReq{Password: &newPw})

	assert.NoError(t, err)
	assert.NotEqual(t, "newsecret", user.Password)
	userRepo.AssertExpectations(t)
}

func TestUserService_UpdateUser_NotFound(t *testing.T) {
	userRepo := new(MockUserRepo)
	jwk := new(MockJwksService)
	refresh := new(MockRefreshTokenService)
	svc := baseUserService(userRepo, jwk, refresh)

	userRepo.On("GetUser", mock.Anything, "u1").Return(nil, errs.ErrNotFound).Once()

	user, err := svc.UpdateUser(context.Background(), "u1", structs.UpdateUserReq{})

	assert.Nil(t, user)
	assert.ErrorIs(t, err, errs.ErrNotFound)
	userRepo.AssertNotCalled(t, "UpdateUser", mock.Anything, mock.Anything)
	userRepo.AssertExpectations(t)
}

func TestUserService_UpdateUser_RepoError(t *testing.T) {
	userRepo := new(MockUserRepo)
	jwk := new(MockJwksService)
	refresh := new(MockRefreshTokenService)
	svc := baseUserService(userRepo, jwk, refresh)

	existing := &domain.User{Id: "u1", Name: "Alice"}
	userRepo.On("GetUser", mock.Anything, "u1").Return(existing, nil).Once()
	userRepo.On("UpdateUser", mock.Anything, mock.Anything).Return(errors.New("db down")).Once()

	user, err := svc.UpdateUser(context.Background(), "u1", structs.UpdateUserReq{})

	assert.Nil(t, user)
	assert.EqualError(t, err, "db down")
	userRepo.AssertExpectations(t)
}

func TestUserService_CreateOAuthUser_Success_NoUow(t *testing.T) {
	userRepo := new(MockUserRepo)
	jwk := new(MockJwksService)
	refresh := new(MockRefreshTokenService)
	svc := baseUserService(userRepo, jwk, refresh)

	user := domain.User{Id: "u1", Name: "Alice", Email: "a@example.com", Provider: "google"}
	userRepo.On("CreateOAuthUser", mock.Anything, user).Return(nil).Once()

	err := svc.CreateOAuthUser(context.Background(), user)

	assert.NoError(t, err)
	userRepo.AssertExpectations(t)
}

func TestUserService_CreateOAuthUser_Success_WithUow(t *testing.T) {
	userRepo := new(MockUserRepo)
	jwtRepo := new(MockJwksRepo)
	jwk := new(MockJwksService)
	refresh := new(MockRefreshTokenService)
	svc := NewUserService(userRepo, jwk, refresh, &MockLogger{}, makeUow(userRepo, jwtRepo))

	user := domain.User{Id: "u1", Name: "Alice", Email: "a@example.com"}
	userRepo.On("CreateOAuthUser", mock.Anything, user).Return(nil).Once()

	err := svc.CreateOAuthUser(context.Background(), user)

	assert.NoError(t, err)
	userRepo.AssertExpectations(t)
}

func TestUserService_CreateOAuthUser_Error(t *testing.T) {
	userRepo := new(MockUserRepo)
	jwk := new(MockJwksService)
	refresh := new(MockRefreshTokenService)
	svc := baseUserService(userRepo, jwk, refresh)

	user := domain.User{Id: "u1"}
	userRepo.On("CreateOAuthUser", mock.Anything, user).Return(errs.ErrConflict).Once()

	err := svc.CreateOAuthUser(context.Background(), user)

	assert.ErrorIs(t, err, errs.ErrConflict)
	userRepo.AssertExpectations(t)
}

func TestUserService_Login_Success(t *testing.T) {
	userRepo := new(MockUserRepo)
	jwk := new(MockJwksService)
	refresh := new(MockRefreshTokenService)
	svc := baseUserService(userRepo, jwk, refresh)

	email := "alice@example.com"
	pw := "secret"
	jkt := "thumbprint"
	user := &domain.User{Id: "u1", Name: "Alice", Email: email, Role: domain.USER, Password: mustHash(t, pw)}
	key := makeSigningKey(t)

	userRepo.On("GetUserByEmail", mock.Anything, email).Return(user, nil).Once()
	jwk.On("GetActiveKey", mock.Anything).Return(key, nil).Once()
	refresh.On("InsertRefreshToken", mock.Anything, mock.MatchedBy(func(tk structs.RefreshToken) bool {
		return tk.Subject == "u1" && tk.RefreshToken != "" && tk.Jkt == jkt
	})).Return(nil).Once()

	tokens, err := svc.Login(context.Background(), email, pw, jkt)

	assert.NoError(t, err)
	assert.NotNil(t, tokens)
	assert.NotEmpty(t, tokens.AccessToken)
	assert.NotEmpty(t, tokens.RefreshToken)
	userRepo.AssertExpectations(t)
	jwk.AssertExpectations(t)
	refresh.AssertExpectations(t)
}

func TestUserService_Login_UserNotFound(t *testing.T) {
	userRepo := new(MockUserRepo)
	jwk := new(MockJwksService)
	refresh := new(MockRefreshTokenService)
	svc := baseUserService(userRepo, jwk, refresh)

	userRepo.On("GetUserByEmail", mock.Anything, "nobody@example.com").Return(nil, errs.ErrNotFound).Once()

	tokens, err := svc.Login(context.Background(), "nobody@example.com", "pw", "jkt")

	assert.Nil(t, tokens)
	assert.ErrorIs(t, err, errs.ErrInvalidCreds)
	jwk.AssertNotCalled(t, "GetActiveKey", mock.Anything)
	userRepo.AssertExpectations(t)
}

func TestUserService_Login_WrongPassword(t *testing.T) {
	userRepo := new(MockUserRepo)
	jwk := new(MockJwksService)
	refresh := new(MockRefreshTokenService)
	svc := baseUserService(userRepo, jwk, refresh)

	email := "alice@example.com"
	user := &domain.User{Id: "u1", Email: email, Password: "hashed-something"}

	userRepo.On("GetUserByEmail", mock.Anything, email).Return(user, nil).Once()

	tokens, err := svc.Login(context.Background(), email, "wrong", "jkt")

	assert.Nil(t, tokens)
	assert.ErrorIs(t, err, errs.ErrInvalidCreds)
	jwk.AssertNotCalled(t, "GetActiveKey", mock.Anything)
	userRepo.AssertExpectations(t)
}

func TestUserService_Login_JwkError(t *testing.T) {
	userRepo := new(MockUserRepo)
	jwk := new(MockJwksService)
	refresh := new(MockRefreshTokenService)
	svc := baseUserService(userRepo, jwk, refresh)

	email := "alice@example.com"
	pw := "secret"
	user := &domain.User{Id: "u1", Email: email, Password: mustHash(t, pw)}

	userRepo.On("GetUserByEmail", mock.Anything, email).Return(user, nil).Once()
	jwk.On("GetActiveKey", mock.Anything).Return(nil, errors.New("no active key")).Once()

	tokens, err := svc.Login(context.Background(), email, pw, "jkt")

	assert.Nil(t, tokens)
	assert.EqualError(t, err, "no active key")
	refresh.AssertNotCalled(t, "InsertRefreshToken", mock.Anything, mock.Anything)
	jwk.AssertExpectations(t)
	userRepo.AssertExpectations(t)
}

func TestUserService_Login_RefreshInsertError(t *testing.T) {
	userRepo := new(MockUserRepo)
	jwk := new(MockJwksService)
	refresh := new(MockRefreshTokenService)
	svc := baseUserService(userRepo, jwk, refresh)

	email := "alice@example.com"
	pw := "secret"
	user := &domain.User{Id: "u1", Email: email, Password: mustHash(t, pw)}
	key := makeSigningKey(t)

	userRepo.On("GetUserByEmail", mock.Anything, email).Return(user, nil).Once()
	jwk.On("GetActiveKey", mock.Anything).Return(key, nil).Once()
	refresh.On("InsertRefreshToken", mock.Anything, mock.Anything).Return(errors.New("redis down")).Once()

	tokens, err := svc.Login(context.Background(), email, pw, "jkt")

	assert.Nil(t, tokens)
	assert.EqualError(t, err, "redis down")
	jwk.AssertExpectations(t)
	refresh.AssertExpectations(t)
	userRepo.AssertExpectations(t)
}
