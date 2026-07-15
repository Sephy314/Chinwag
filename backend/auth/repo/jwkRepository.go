package repo

import (
	"context"
	"database/sql"
	"time"

	"fmt"

	"github.com/Sephy314/chinwag/auth/domain"
	"github.com/jmoiron/sqlx"
)

type JwksRepository interface {
	Load(context.Context) ([]domain.SigningKeyEntity, error)
	Rotate(context.Context, domain.SigningKeyEntity) error
	InActiveKey(context.Context, string) error
	ExpireActiveKey(context.Context) error
	ClearRetiredKeys(context.Context) error
	GetActiveKey(context.Context) (*domain.SigningKeyEntity, error)
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
				WHERE status = 'ACTIVE'`,
	)
	if err != nil {
		return err
	}

	_, err = tx.ExecContext(
		ctx,
		`INSERT INTO signing_keys
			(kid, public_key, private_key, status, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6)`,
		signingKey.Kid,
		signingKey.PublicKey,
		signingKey.PrivateKey,
		signingKey.Status,
		signingKey.CreatedAt,
		signingKey.UpdatedAt,
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
	if err != nil {
		return err
	}

	return nil
}

func (repo *JwksRepo) ExpireActiveKey(ctx context.Context) error {
	_, err := repo.db.ExecContext(
		ctx,
		`UPDATE signing_keys
			   SET status = 'EXPIRED'
			   WHERE status = 'ACTIVE'`,
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

func (repo *JwksRepo) GetActiveKey(ctx context.Context) (*domain.SigningKeyEntity, error) {
	var signingKey domain.SigningKeyEntity

	err := sqlx.GetContext(
		ctx,
		repo.db,
		&signingKey,
		"SELECT * FROM signing_keys WHERE status = 'ACTIVE' LIMIT 1",
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
