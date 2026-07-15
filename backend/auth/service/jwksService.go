package service

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"log"
	"time"

	"github.com/Sephy314/chinwag/auth/domain"
	"github.com/Sephy314/chinwag/auth/repo"
	"github.com/Sephy314/chinwag/shared/errs"
	"github.com/Sephy314/chinwag/shared/utils"
	"github.com/google/uuid"
	"github.com/lestrrat-go/jwx/v3/jwk"
)

type JwksServiceInterface interface {
	LoadJWKS(ctx context.Context) error
	GetJwkSet(ctx context.Context) (jwk.Set, error)
	Rotate(ctx context.Context) error
	GetActiveKey(ctx context.Context) (*domain.SigningKey, error)
}

type JwksService struct {
	jwkSet  jwk.Set
	repo    repo.JwksRepository
	version time.Time
}

func NewJwksService(repo repo.JwksRepository) *JwksService {
	s := &JwksService{
		jwkSet: jwk.NewSet(),
		repo:   repo,
	}

	err := s.LoadJWKS(context.Background())
	if err != nil {
		log.Fatal(err)
	}

	return s
}

func (s *JwksService) LoadJWKS(ctx context.Context) error {
	dbVersion, err := s.repo.GetVersion(ctx)
	if err != nil {
		return err
	}

	if dbVersion == nil {
		err = s.Rotate(ctx)
		if err != nil {
			return err
		}

		dbVersion, err = s.repo.GetVersion(ctx)
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

	for _, k := range keys {
		if k.Status == domain.Active && k.ExpiredAt != nil && time.Now().After(*k.ExpiredAt) {
			err = s.repo.ExpireActiveKey(ctx)
			if err != nil {
				return err
			}

			err = s.Rotate(ctx)
			if err != nil {
				return err
			}

			keys, err = s.repo.Load(ctx)
			if err != nil {
				return err
			}

			break
		}
	}

	set, err := utils.ToJWKS(keys)
	if err != nil {
		return err
	}

	s.jwkSet = set
	s.version = *dbVersion

	return nil
}

func (s *JwksService) GetJwkSet(ctx context.Context) (jwk.Set, error) {
	err := s.LoadJWKS(ctx)
	if err != nil {
		return nil, err
	}

	return s.jwkSet, nil
}

func (s *JwksService) Rotate(ctx context.Context) error {
	newKid := uuid.Must(uuid.NewV7()).String()

	newPriv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)

	if err != nil {
		return err
	}

	serialisedPub, err := utils.EncodePublicKey(new(newPriv.PublicKey))
	if err != nil {
		return err
	}
	serialisedPriv, err := utils.EncodePrivateKey(newPriv)
	if err != nil {
		return err
	}

	now := time.Now()

	newKey := domain.SigningKeyEntity{
		Kid:        newKid,
		PublicKey:  serialisedPub,
		PrivateKey: serialisedPriv,
		Status:     domain.Active,
		CreatedAt:  now,
		UpdatedAt:  new(now),
		ExpiredAt:  new(utils.GetExpiredAt(now)),
	}

	err = s.repo.Rotate(ctx, newKey)
	return err
}

func (s *JwksService) GetActiveKey(ctx context.Context) (*domain.SigningKey, error) {
	key, e := s.repo.GetActiveKey(ctx)
	if e != nil {
		return nil, e
	}

	return utils.SigningKeyEntityToSigningKey(*key)

}

func (s *JwksService) GetPublicKey(ctx context.Context, kid string) (*ecdsa.PublicKey, error) {
	err := s.LoadJWKS(ctx)
	if err != nil {
		return nil, err
	}

	foundJwk, o := s.jwkSet.LookupKeyID(kid)

	if !o {
		return nil, errs.ErrNoKey
	}

	var pub ecdsa.PublicKey
	if err := jwk.Export(foundJwk, &pub); err != nil {
		return nil, err
	}

	return &pub, nil

}
