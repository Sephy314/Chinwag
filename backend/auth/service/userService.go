package service

import (
	"context"

	"github.com/Sephy314/chinwag/auth/domain"
	"github.com/Sephy314/chinwag/auth/repo"
	"github.com/Sephy314/chinwag/auth/structs"
	"github.com/Sephy314/chinwag/conn/cache"
	"github.com/Sephy314/chinwag/shared/errs"
	"github.com/Sephy314/chinwag/shared/patch"
	"github.com/Sephy314/chinwag/shared/utils"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

type UserService struct {
	Repo           repo.UserRepository
	Cache          cache.Cache
	JwkService     JwksServiceInterface
	RefreshService RefreshTokenServiceInterface
	uow            repo.UnitOfWork
}

func NewUserService(
	cache cache.Cache,
	userRepo repo.UserRepository,
	jwkService JwksServiceInterface,
	refreshService RefreshTokenServiceInterface,
	uow ...repo.UnitOfWork,
) *UserService {
	var unitOfWork repo.UnitOfWork
	if len(uow) > 0 {
		unitOfWork = uow[0]
	}

	return &UserService{
		Repo:           userRepo,
		Cache:          cache,
		JwkService:     jwkService,
		RefreshService: refreshService,
		uow:            unitOfWork,
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

	if s.uow == nil {
		err = s.Repo.CreateUser(ctx, *newUser)
		if err != nil {
			return nil, err
		}
		return newUser, nil
	}

	err = s.uow.Do(ctx, func(txCtx context.Context, tx repo.Transaction) error {
		return tx.UserRepo().CreateUser(txCtx, *newUser)
	})
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
	if s.uow == nil {
		return s.Repo.DeleteUser(ctx, id)
	}

	return s.uow.Do(ctx, func(txCtx context.Context, tx repo.Transaction) error {
		return tx.UserRepo().DeleteUser(txCtx, id)
	})
}

func (s *UserService) UpdateUser(ctx context.Context, id string, req structs.UpdateUserReq) (*domain.User, error) {
	user, err := s.Repo.GetUser(ctx, id)
	if err != nil {
		return nil, err
	}

	_, err = patch.Patch(user, req,
		patch.WithIgnore("CreatedAt", "UpdatedAt", "DeletedAt"),
		patch.WithTransform("Password", func(name string, value any) (any, error) {
			hash, err := bcrypt.GenerateFromPassword([]byte(value.(string)), bcrypt.DefaultCost)
			if err != nil {
				return nil, err
			}
			return string(hash), nil
		}),
	)
	if err != nil {
		return nil, err
	}

	if s.uow == nil {
		err = s.Repo.UpdateUser(ctx, *user)
		if err != nil {
			return nil, err
		}
		return user, nil
	}

	err = s.uow.Do(ctx, func(txCtx context.Context, tx repo.Transaction) error {
		return tx.UserRepo().UpdateUser(txCtx, *user)
	})
	if err != nil {
		return nil, err
	}

	return user, nil
}

func (s *UserService) Login(ctx context.Context, email string, pw string) (*structs.TokenSet, error) {
	user, err := s.Repo.GetUserByEmail(ctx, email)
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

	accessToken, err := utils.NewToken(user.Id, user.Role, key.PrivateKey, key.Kid)
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
