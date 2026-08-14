package repo

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/Sephy314/chinwag/backend/services/auth/domain"
	"github.com/jmoiron/sqlx"
)

type JwksRepository interface {
	Load(context.Context) ([]domain.SigningKeyEntity, error)
	Rotate(context.Context, domain.SigningKeyEntity) error
	InActiveKey(context.Context, string) error
	ExpireActiveKey(context.Context, domain.KeyType) error
	ClearRetiredKeys(context.Context) error
	GetActiveKey(context.Context, domain.KeyType) (*domain.SigningKeyEntity, error)
	GetKeyByKid(context.Context, string) (*domain.SigningKeyEntity, error)
	GetVersion(context.Context) (*time.Time, error)
}

type JwksRepo struct {
	db sqlx.ExtContext
}

func NewJwtRepository(db sqlx.ExtContext) JwksRepository {
	return &JwksRepo{db: db}
}

func (repo *JwksRepo) Load(ctx context.Context) ([]domain.SigningKeyEntity, error) {
	var signingKeys []domain.SigningKeyEntity

	err := sqlx.SelectContext(
		ctx,
		repo.db,
		&signingKeys,
		`
		SELECT
			kid,
			key_type,
			public_key,
			private_key,
			status,
			created_at,
			updated_at,
			expired_at
		FROM signing_keys
		WHERE status IN ('ACTIVE', 'INACTIVE')
		`,
	)
	if err != nil {
		return nil, err
	}

	if signingKeys == nil {
		return []domain.SigningKeyEntity{}, nil
	}

	return signingKeys, nil
}

func (repo *JwksRepo) Rotate(
	ctx context.Context,
	signingKey domain.SigningKeyEntity,
) error {
	var tx *sqlx.Tx
	var err error
	ownsTx := false

	switch d := repo.db.(type) {
	case *sqlx.DB:
		tx, err = d.BeginTxx(ctx, nil)
		if err != nil {
			return err
		}
		ownsTx = true
		defer func() {
			if err != nil {
				_ = tx.Rollback()
			}
		}()
	case *sqlx.Tx:
		tx = d
	default:
		return fmt.Errorf("unsupported db type for transaction")
	}

	_, err = tx.ExecContext(
		ctx,
		`UPDATE signing_keys
				SET status = 'INACTIVE'
				WHERE status = 'ACTIVE' AND key_type = $1`,
		signingKey.Type,
	)
	if err != nil {
		return err
	}

	_, err = tx.ExecContext(
		ctx,
		`INSERT INTO signing_keys
			(kid, key_type, public_key, private_key, status, created_at, updated_at, expired_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		signingKey.Kid,
		signingKey.Type,
		signingKey.PublicKey,
		signingKey.PrivateKey,
		signingKey.Status,
		signingKey.CreatedAt,
		signingKey.UpdatedAt,
		signingKey.ExpiredAt,
	)
	if err != nil {
		return err
	}

	if ownsTx {
		return tx.Commit()
	}
	return nil
}

func (repo *JwksRepo) InActiveKey(ctx context.Context, kid string) error {
	_, err := repo.db.ExecContext(
		ctx,
		`UPDATE signing_keys 
			   SET status = 'INACTIVE'
			   WHERE kid = $1`,
		kid,
	)
	return err
}

func (repo *JwksRepo) ExpireActiveKey(ctx context.Context, keyType domain.KeyType) error {
	_, err := repo.db.ExecContext(
		ctx,
		`UPDATE signing_keys
			   SET status = 'EXPIRED'
			   WHERE status = 'ACTIVE' AND key_type = $1`,
		keyType,
	)
	return err
}

func (repo *JwksRepo) ClearRetiredKeys(ctx context.Context) error {
	_, err := repo.db.ExecContext(
		ctx,
		"DELETE FROM signing_keys WHERE status = 'RETIRED'",
	)
	return err
}

func (repo *JwksRepo) GetActiveKey(ctx context.Context, keyType domain.KeyType) (*domain.SigningKeyEntity, error) {
	var signingKey domain.SigningKeyEntity

	err := sqlx.GetContext(
		ctx,
		repo.db,
		&signingKey,
		"SELECT * FROM signing_keys WHERE status = 'ACTIVE' AND key_type = $1 LIMIT 1",
		keyType,
	)
	if err != nil {
		return nil, err
	}

	return &signingKey, nil
}

// GetKeyByKid loads a signing key regardless of its current status so that
// tokens signed with a rotated (now INACTIVE) key can still be verified for
// the remainder of their lifetime.
func (repo *JwksRepo) GetKeyByKid(ctx context.Context, kid string) (*domain.SigningKeyEntity, error) {
	var signingKey domain.SigningKeyEntity

	err := sqlx.GetContext(
		ctx,
		repo.db,
		&signingKey,
		"SELECT * FROM signing_keys WHERE kid = $1 LIMIT 1",
		kid,
	)
	if err != nil {
		return nil, err
	}

	return &signingKey, nil
}

func (repo *JwksRepo) GetVersion(ctx context.Context) (*time.Time, error) {
	var version sql.NullTime

	err := sqlx.GetContext(
		ctx,
		repo.db,
		&version,
		"SELECT MAX(updated_at) FROM signing_keys",
	)
	if err != nil {
		return nil, err
	}

	if !version.Valid {
		return nil, nil
	}

	return &version.Time, nil
}
