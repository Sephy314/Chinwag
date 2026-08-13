package service

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"database/sql"
	"errors"
	"time"

	"github.com/Sephy314/chinwag/backend/services/auth/domain"
	"github.com/Sephy314/chinwag/backend/services/auth/internal/jwt"
	"github.com/Sephy314/chinwag/backend/services/auth/repo"
	"github.com/Sephy314/chinwag/backend/services/auth/shared/errs"
	"github.com/Sephy314/chinwag/backend/services/auth/shared/logger"
	"github.com/google/uuid"
	"github.com/lestrrat-go/jwx/v3/jwk"
)

type JwksServiceInterface interface {
	LoadJWKS(ctx context.Context) error
	GetJwkSet(ctx context.Context) (jwk.Set, error)
	RotateAccess(ctx context.Context) error
	RotateRefresh(ctx context.Context) error
	GetActiveAccessKey(ctx context.Context) (*domain.SigningKey, error)
	GetActiveRefreshKey(ctx context.Context) (*domain.SigningKey, error)
	GetPublicKey(ctx context.Context, kid string) (*ecdsa.PublicKey, error)
	GetRefreshKeyByKid(ctx context.Context, kid string) (*domain.SigningKey, error)
}

type JwksService struct {
	jwkSet  jwk.Set
	repo    repo.JwksRepository
	version time.Time
	log     logger.Logger
}

func NewJwksService(repo repo.JwksRepository, log logger.Logger) *JwksService {
	s := &JwksService{
		jwkSet: jwk.NewSet(),
		repo:   repo,
		log:    log,
	}

	err := s.LoadJWKS(context.Background())
	if err != nil {
		log.Fatal("failed to load JWKS", "error", err)
	}

	return s
}

func (s *JwksService) LoadJWKS(ctx context.Context) error {
	dbVersion, err := s.repo.GetVersion(ctx)
	if err != nil {
		return err
	}

	if dbVersion == nil {
		// First boot: seed one active key per type (Access + Refresh).
		err = s.ensureKeys(ctx)
		if err != nil {
			return err
		}

		dbVersion, err = s.repo.GetVersion(ctx)
		if err != nil {
			return err
		}
	}

	if dbVersion == nil {
		return nil
	}

	if !dbVersion.After(s.version) && len(s.jwkSet.Keys()) > 0 {
		return nil
	}

	keys, err := s.repo.Load(ctx)
	if err != nil {
		return err
	}

	// Access and Refresh key lifecycles are managed independently: expire and
	// rotate each type on its own schedule, and backfill a missing type. Reload
	// once afterwards if any key was rotated.
	rotated := false
	for _, kt := range []domain.KeyType{domain.KeyTypeAccess, domain.KeyTypeRefresh} {
		didRotate, err := s.rotateIfNeeded(ctx, keys, kt)
		if err != nil {
			return err
		}
		rotated = rotated || didRotate
	}
	if rotated {
		keys, err = s.repo.Load(ctx)
		if err != nil {
			return err
		}
	}

	set, err := jwt.ToJWKSet(keys)
	if err != nil {
		return err
	}

	s.jwkSet = set
	s.version = *dbVersion

	return nil
}

// ensureKeys creates an active key for any type that has none (first boot /
// post-migration backfill). Used when there is no version row at all.
func (s *JwksService) ensureKeys(ctx context.Context) error {
	for _, kt := range []domain.KeyType{domain.KeyTypeAccess, domain.KeyTypeRefresh} {
		if _, err := s.repo.GetActiveKey(ctx, kt); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				if err := s.rotate(ctx, kt); err != nil {
					return err
				}
				continue
			}
			return err
		}
	}
	return nil
}

// rotateIfNeeded expires and rotates the active key of kt when it has expired,
// or backfills a missing active key of that type. It reports whether a rotation
// happened so the caller can decide whether to reload the key set.
func (s *JwksService) rotateIfNeeded(ctx context.Context, keys []domain.SigningKeyEntity, kt domain.KeyType) (bool, error) {
	var active *domain.SigningKeyEntity
	for i := range keys {
		k := &keys[i]
		if k.Type != kt {
			continue
		}
		if k.Status == domain.Active {
			active = k
			break
		}
	}

	if active == nil {
		return true, s.rotate(ctx, kt)
	}

	if active.ExpiredAt != nil && time.Now().After(*active.ExpiredAt) {
		if err := s.repo.ExpireActiveKey(ctx, kt); err != nil {
			return false, err
		}
		return true, s.rotate(ctx, kt)
	}

	return false, nil
}

func (s *JwksService) GetJwkSet(ctx context.Context) (jwk.Set, error) {
	err := s.LoadJWKS(ctx)
	if err != nil {
		return nil, err
	}

	return s.jwkSet, nil
}

func (s *JwksService) rotate(ctx context.Context, keyType domain.KeyType) error {
	newKid := uuid.Must(uuid.NewV7()).String()

	newPriv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return err
	}

	serialisedPub, err := jwt.EncodePublicKey(&newPriv.PublicKey)
	if err != nil {
		return err
	}
	serialisedPriv, err := jwt.EncodePrivateKey(newPriv)
	if err != nil {
		return err
	}

	now := time.Now()

	newKey := domain.SigningKeyEntity{
		Kid:        newKid,
		Type:       keyType,
		PublicKey:  serialisedPub,
		PrivateKey: serialisedPriv,
		Status:     domain.Active,
		CreatedAt:  now,
		UpdatedAt:  &now,
		ExpiredAt:  ptrTime(now.Add(time.Hour * 24)),
	}

	err = s.repo.Rotate(ctx, newKey)
	return err
}

func (s *JwksService) RotateAccess(ctx context.Context) error {
	return s.rotate(ctx, domain.KeyTypeAccess)
}

func (s *JwksService) RotateRefresh(ctx context.Context) error {
	return s.rotate(ctx, domain.KeyTypeRefresh)
}

func (s *JwksService) GetActiveAccessKey(ctx context.Context) (*domain.SigningKey, error) {
	key, e := s.repo.GetActiveKey(ctx, domain.KeyTypeAccess)
	if e != nil {
		return nil, e
	}

	return jwt.SigningKeyEntityToSigningKey(*key)
}

func (s *JwksService) GetActiveRefreshKey(ctx context.Context) (*domain.SigningKey, error) {
	key, e := s.repo.GetActiveKey(ctx, domain.KeyTypeRefresh)
	if e != nil {
		return nil, e
	}

	return jwt.SigningKeyEntityToSigningKey(*key)
}

// GetRefreshKeyByKid loads the signing key referenced by a refresh-token JWT's
// kid and asserts it is a Refresh-type key. Rotated (INACTIVE) keys remain
// loadable so in-flight refresh tokens keep verifying until they expire.
func (s *JwksService) GetRefreshKeyByKid(ctx context.Context, kid string) (*domain.SigningKey, error) {
	ent, err := s.repo.GetKeyByKid(ctx, kid)
	if err != nil {
		return nil, err
	}
	if ent.Type != domain.KeyTypeRefresh {
		return nil, errs.ErrNoKey
	}
	return jwt.SigningKeyEntityToSigningKey(*ent)
}

func (s *JwksService) GetPublicKey(ctx context.Context, kid string) (*ecdsa.PublicKey, error) {
	err := s.LoadJWKS(ctx)
	if err != nil {
		return nil, err
	}

	foundJwk, ok := s.jwkSet.LookupKeyID(kid)
	if !ok {
		return nil, errs.ErrNoKey
	}

	var pub ecdsa.PublicKey
	if err := jwk.Export(foundJwk, &pub); err != nil {
		return nil, err
	}

	return &pub, nil
}

func ptrTime(t time.Time) *time.Time {
	return &t
}
