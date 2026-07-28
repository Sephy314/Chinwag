package service

import (
	"context"

	"github.com/Sephy314/chinwag/backend/services/auth/domain"
	"github.com/Sephy314/chinwag/backend/services/auth/internal/jwt"
	"github.com/Sephy314/chinwag/backend/services/auth/repo"
	"github.com/Sephy314/chinwag/backend/services/auth/shared/errs"
	"github.com/Sephy314/chinwag/backend/services/auth/shared/logger"
	"github.com/Sephy314/chinwag/backend/services/auth/structs"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

type UserService struct {
	Repo           repo.UserRepository
	Cache          interface{ Get(ctx context.Context, key string) (string, error) }
	JwkService     JwksServiceInterface
	RefreshService RefreshTokenServiceInterface
	uow            repo.UnitOfWork
	log            logger.Logger
}

func NewUserService(
	userRepo repo.UserRepository,
	jwkService JwksServiceInterface,
	refreshService RefreshTokenServiceInterface,
	log logger.Logger,
	uow ...repo.UnitOfWork,
) *UserService {
	var unitOfWork repo.UnitOfWork
	if len(uow) > 0 {
		unitOfWork = uow[0]
	}

	return &UserService{
		Repo:           userRepo,
		JwkService:     jwkService,
		RefreshService: refreshService,
		uow:            unitOfWork,
		log:            log,
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
		Name:     req.Name,
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

	if req.Name != nil {
		user.Name = *req.Name
	}
	if req.Email != nil {
		user.Email = *req.Email
	}
	if req.Password != nil {
		hash, err := bcrypt.GenerateFromPassword([]byte(*req.Password), bcrypt.DefaultCost)
		if err != nil {
			return nil, err
		}
		user.Password = string(hash)
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

func (s *UserService) CreateOAuthUser(ctx context.Context, user domain.User) error {
	if s.uow == nil {
		return s.Repo.CreateOAuthUser(ctx, user)
	}

	return s.uow.Do(ctx, func(txCtx context.Context, tx repo.Transaction) error {
		return tx.UserRepo().CreateOAuthUser(txCtx, user)
	})
}

func (s *UserService) Login(ctx context.Context, email string, pw string) (*structs.TokenSet, error) {
	s.log.Info("login attempt", "email", email)

	user, err := s.Repo.GetUserByEmail(ctx, email)
	if err != nil {
		s.log.Warn("login failed: user not found", "email", email)
		return nil, errs.ErrInvalidCreds
	}

	if err = bcrypt.CompareHashAndPassword(
		[]byte(user.Password),
		[]byte(pw),
	); err != nil {
		s.log.Warn("login failed: wrong password", "email", email)
		return nil, errs.ErrInvalidCreds
	}

	key, err := s.JwkService.GetActiveKey(ctx)
	if err != nil {
		s.log.Error("login failed: could not get active key", "email", email, "error", err)
		return nil, err
	}

	accessToken, err := jwt.Sign(user.Id, string(user.Role), key.PrivateKey, key.Kid)
	if err != nil {
		s.log.Error("login failed: could not generate token", "email", email, "error", err)
		return nil, err
	}

	refreshToken := uuid.Must(uuid.NewV7()).String()

	err = s.RefreshService.InsertRefreshToken(ctx, structs.RefreshToken{
		Subject:      user.Id,
		RefreshToken: refreshToken,
	})
	if err != nil {
		s.log.Error("login failed: could not insert refresh token", "email", email, "error", err)
		return nil, err
	}

	s.log.Info("login successful", "email", email, "userId", user.Id)

	return &structs.TokenSet{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
	}, nil
}
