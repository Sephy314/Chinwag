package service

import (
	"context"
	"database/sql"
	"errors"

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
	Repo  repo.UserRepository
	Cache interface {
		Get(ctx context.Context, key string) (string, error)
	}
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

func (s *UserService) Login(ctx context.Context, email string, pw string, jkt string) (*structs.TokenSet, error) {
	s.log.Info("login attempt", "email", email)

	user, err := s.Repo.GetUserByEmail(ctx, email)
	if err != nil {
		// Only mask "no such user" as invalid credentials. Infrastructure
		// errors (DB down, timeouts, broken connections) must propagate so the
		// handler can return 500 instead of a misleading 400.
		if errors.Is(err, sql.ErrNoRows) {
			s.log.Warn("login failed: user not found", "email", email)
			return nil, errs.ErrInvalidCreds
		}
		s.log.Error("login failed: could not load user", "email", email, "error", err)
		return nil, err
	}

	if err = bcrypt.CompareHashAndPassword(
		[]byte(user.Password),
		[]byte(pw),
	); err != nil {
		s.log.Warn("login failed: wrong password", "email", email)
		return nil, errs.ErrInvalidCreds
	}

	key, err := s.JwkService.GetActiveAccessKey(ctx)
	if err != nil {
		s.log.Error("login failed: could not get active key", "email", email, "error", err)
		return nil, err
	}

	accessToken, err := jwt.SignWithCNF(user.Id, string(user.Role), key.PrivateKey, key.Kid, jkt)
	if err != nil {
		s.log.Error("login failed: could not generate token", "email", email, "error", err)
		return nil, err
	}

	refreshToken, err := s.RefreshService.IssueRefreshToken(ctx, user.Id, jkt, "")
	if err != nil {
		s.log.Error("login failed: could not issue refresh token", "email", email, "error", err)
		return nil, err
	}

	s.log.Info("login successful", "email", email, "user_id", user.Id)

	return &structs.TokenSet{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		UserId:       user.Id,
	}, nil
}

// --- Admin operations ---

func toAdminUser(u domain.User) structs.AdminUserResponse {
	return structs.AdminUserResponse{
		Id:        u.Id,
		Name:      u.Name,
		Email:     u.Email,
		Role:      string(u.Role),
		Provider:  u.Provider,
		CreatedAt: u.CreatedAt,
		UpdatedAt: u.UpdatedAt,
		DeletedAt: u.DeletedAt,
	}
}

func (s *UserService) AdminListUsers(ctx context.Context, req structs.ListUsersRequest) ([]structs.AdminUserResponse, *structs.CursorMeta, error) {
	if req.Limit <= 0 || req.Limit > 200 {
		req.Limit = 50
	}
	users, meta, err := s.Repo.ListUsers(ctx, req.Cursor, req.Limit, req.Role, req.Deleted, req.Search)
	if err != nil {
		return nil, nil, err
	}
	out := make([]structs.AdminUserResponse, len(users))
	for i, u := range users {
		out[i] = toAdminUser(u)
	}
	return out, meta, nil
}

func (s *UserService) AdminGetUser(ctx context.Context, id string) (*structs.AdminUserResponse, error) {
	u, err := s.Repo.GetUserIncludingDeleted(ctx, id)
	if err != nil {
		return nil, err
	}
	user := toAdminUser(*u)
	return &user, nil
}

func (s *UserService) AdminCreateUser(ctx context.Context, req structs.CreateAdminUserRequest) (*structs.AdminUserResponse, error) {
	role := req.Role
	if role == "" {
		role = string(domain.USER)
	}
	if !validRole(role) {
		return nil, errs.ErrInvalidRole
	}

	u, err := s.CreateUser(ctx, structs.CreateUserReq{
		Name:     req.Name,
		Email:    req.Email,
		Password: req.Password,
	})
	if err != nil {
		return nil, err
	}
	if role != string(domain.USER) {
		if err := s.Repo.SetRole(ctx, u.Id, domain.Role(role)); err != nil {
			return nil, err
		}
	}
	admin := toAdminUser(*u)
	admin.Role = role
	return &admin, nil
}

func (s *UserService) AdminUpdateUser(ctx context.Context, id string, req structs.UpdateAdminUserRequest) (*structs.AdminUserResponse, error) {
	u, err := s.UpdateUser(ctx, id, structs.UpdateUserReq{
		Name:     req.Name,
		Email:    req.Email,
		Password: req.Password,
	})
	if err != nil {
		return nil, err
	}
	admin := toAdminUser(*u)
	return &admin, nil
}

func (s *UserService) AdminSetRole(ctx context.Context, id, actorID, role string) error {
	if !validRole(role) {
		return errs.ErrInvalidRole
	}
	if id == actorID && role != string(domain.ADMIN) {
		return errs.ErrSelfDemotion
	}

	current, err := s.Repo.GetUserIncludingDeleted(ctx, id)
	if err != nil {
		return err
	}
	if string(current.Role) == string(domain.ADMIN) && role != string(domain.ADMIN) {
		n, err := s.Repo.CountAdmins(ctx)
		if err != nil {
			return err
		}
		if n <= 1 {
			return errs.ErrLastAdmin
		}
	}

	return s.Repo.SetRole(ctx, id, domain.Role(role))
}

func (s *UserService) AdminDisableUser(ctx context.Context, id string) error {
	current, err := s.Repo.GetUserIncludingDeleted(ctx, id)
	if err != nil {
		return err
	}
	// Disabling an ADMIN effectively removes an admin; protect the last one.
	if string(current.Role) == string(domain.ADMIN) {
		n, err := s.Repo.CountAdmins(ctx)
		if err != nil {
			return err
		}
		if n <= 1 {
			return errs.ErrLastAdmin
		}
	}
	return s.DeleteUser(ctx, id)
}

func (s *UserService) AdminRestoreUser(ctx context.Context, id string) (*structs.AdminUserResponse, error) {
	if err := s.Repo.RestoreUser(ctx, id); err != nil {
		return nil, err
	}
	u, err := s.Repo.GetUserIncludingDeleted(ctx, id)
	if err != nil {
		return nil, err
	}
	admin := toAdminUser(*u)
	return &admin, nil
}

func (s *UserService) CountUsers(ctx context.Context) (int64, error) {
	n, err := s.Repo.CountUsers(ctx)
	return int64(n), err
}

func validRole(role string) bool {
	switch role {
	case string(domain.USER), string(domain.MANAGER), string(domain.ADMIN):
		return true
	default:
		return false
	}
}
