package service

import (
	"context"

	"github.com/Sephy314/chinwag/auth/domain"
	"github.com/Sephy314/chinwag/auth/errs"
	"github.com/Sephy314/chinwag/auth/repo"
	"github.com/Sephy314/chinwag/auth/structs"
	"github.com/Sephy314/chinwag/auth/utils"
	"github.com/Sephy314/chinwag/conn/cache"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

type UserService struct {
	Repo           repo.UserRepository
	Cache          cache.Cache
	JwkService     JwksServiceImpl
	RefreshService RefreshTokenServiceImpl
}

func NewUserService(
	cache cache.Cache,
	repo repo.UserRepository,
	jwkService JwksServiceImpl,
	refreshService RefreshTokenServiceImpl,
) *UserService {
	return &UserService{
		Repo:           repo,
		Cache:          cache,
		JwkService:     jwkService,
		RefreshService: refreshService,
	}
}

func (s *UserService) CreateUser(ctx context.Context, req structs.CreateUserReq) (*domain.User, error) {
	id := uuid.Must(uuid.NewV7()).String()

	hash, err := bcrypt.GenerateFromPassword(
		[]byte(req.Password),
		bcrypt.DefaultCost,
	)

	if err != nil {
		return nil, err
	}

	newUser := &domain.User{
		Id:       id,
		Name:     req.Username,
		Password: string(hash),
		Email:    req.Email,
	}

	err = s.Repo.CreateUser(ctx, *newUser)

	if err != nil {
		return nil, err
	}

	return newUser, nil
}

func (s *UserService) GetUser(ctx context.Context, id string) (*domain.User, error) {
	user, err := s.Repo.GetUser(ctx, id)
	if err != nil {
		return nil, err
	}
	return user, nil
}

func (s *UserService) GetUserByEmail(ctx context.Context, email string) (*domain.User, error) {
	return s.Repo.GetUserByEmail(ctx, email)
}

func (s *UserService) DeleteUser(ctx context.Context, id string) error {
	err := s.Repo.DeleteUser(ctx, id)
	return err
}

func (s *UserService) Login(ctx context.Context, email string, pw string) (*structs.TokenSet, error) {
	user, err := s.Repo.GetUser(ctx, email)
	if err != nil {
		return nil, errs.ErrInvalidCreds
	}

	if err = bcrypt.CompareHashAndPassword(
		[]byte(user.Password),
		[]byte(pw),
	); err != nil {
		return nil, errs.ErrInvalidCreds
	}

	key, err := s.JwkService.GetActiveKey(ctx)

	if err != nil {
		return nil, err
	}

	accessToken, err := utils.NewToken(user.Id, key.PrivateKey)
	if err != nil {
		return nil, err
	}

	refreshToken := uuid.Must(uuid.NewV7()).String()

	err = s.RefreshService.InsertRefreshToken(ctx, structs.RefreshToken{
		Subject:      user.Id,
		RefreshToken: refreshToken,
	})

	if err != nil {
		return nil, err
	}

	keyPair := structs.TokenSet{
		AccessToken:  *accessToken,
		RefreshToken: refreshToken,
	}

	return &keyPair, nil
}
