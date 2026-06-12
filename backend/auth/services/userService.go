package services

import (
	"context"

	"github.com/Sephy314/chinwag/auth/domain"
	"github.com/Sephy314/chinwag/auth/repo"
	"github.com/Sephy314/chinwag/auth/structs"
	"github.com/Sephy314/chinwag/conn/cache"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

type UserService struct {
	Repo  repo.UserRepository
	Cache cache.Cache
}

func NewUserService(cache cache.Cache, repo repo.UserRepository) *UserService {
	return &UserService{
		Repo:  repo,
		Cache: cache,
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
