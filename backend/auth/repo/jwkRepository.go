package repo

import (
	"context"
	"time"

	"github.com/Sephy314/chinwag/auth/domain"
	"github.com/Sephy314/chinwag/conn"
	"github.com/jmoiron/sqlx"
)

type JwtRepository interface {
	Load(context.Context) ([]domain.SigningKeyEntity, error)
	Rotate(context.Context, domain.SigningKeyEntity) error
	InActiveKey(context.Context, string) error
	ClearRetiredKeys(context.Context) error
	GetActiveKey(context.Context) (*domain.SigningKeyEntity, error)
	GetVersion(context.Context) (*time.Time, error)
	Count(context.Context) (*int64, error)
}

type JwtRepo struct {
	db *sqlx.DB
}

func NewJwtRepository(conn *conn.Connection) JwtRepository {
	repo := JwtRepo{
		db: conn.DB,
	}

	return &repo
}

func (repo *JwtRepo) Load(ctx context.Context) ([]domain.SigningKeyEntity, error) {
	var signingKeys []domain.SigningKeyEntity

	err := repo.db.SelectContext(
		ctx,
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

func (repo *JwtRepo) Rotate(
	ctx context.Context,
	signingKey domain.SigningKeyEntity,
) error {
	tx, err := repo.db.BeginTxx(ctx, nil)
	if err != nil {
		return err
	}

	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	_, err = tx.ExecContext(
		ctx,
		"UPDATE signing_keys SET status = 'INACTIVE' WHERE status = 'ACTIVE'",
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

	err = tx.Commit()
	if err != nil {
		return err
	}

	return nil
}

func (repo *JwtRepo) InActiveKey(ctx context.Context, kid string) error {
	_, err := repo.db.ExecContext(
		ctx,
		"UPDATE signing_keys SET status = 'INACTIVE' WHERE kid = $1",
		kid,
	)
	if err != nil {
		return err
	}

	return nil
}

func (repo *JwtRepo) ClearRetiredKeys(ctx context.Context) error {
	_, err := repo.db.ExecContext(
		ctx,
		"DELETE FROM signing_keys WHERE status = 'RETIRED'",
	)

	return err
}

func (repo *JwtRepo) GetActiveKey(ctx context.Context) (*domain.SigningKeyEntity, error) {
	var signingKey domain.SigningKeyEntity

	err := repo.db.GetContext(
		ctx,
		&signingKey,
		"SELECT * FROM signing_keys WHERE status = 'ACTIVE' LIMIT 1",
	)

	if err != nil {
		return nil, err
	}

	return &signingKey, nil
}

func (repo *JwtRepo) GetVersion(ctx context.Context) (*time.Time, error) {
	var version time.Time

	err := repo.db.GetContext(
		ctx,
		&version,
		"SELECT MAX(updated_at) FROM signing_keys",
	)

	if err != nil {
		return nil, err
	}

	return &version, nil
}

func (repo *JwtRepo) Count(ctx context.Context) (*int64, error) {
	var count int64
	err := repo.db.GetContext(
		ctx,
		&count,
		"SELECT COUNT(*) FROM signing_keys",
	)

	if err != nil {
		return nil, err
	}

	return &count, nil
}
