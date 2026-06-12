package conn

import (
	"context"
	"os"

	"github.com/jmoiron/sqlx"
	"github.com/redis/go-redis/v9"
)

import _ "github.com/jackc/pgx/v5/stdlib"

func NewConnection() (*Connection, error) {
	dsn := os.Getenv("DB_DSN_CHINWAG")
	pw := os.Getenv("DB_PW")

	db, err := sqlx.Connect("pgx", dsn)
	if err != nil {
		return nil, err
	}

	rds := redis.NewClient(&redis.Options{
		Addr:     "localhost:6379",
		Password: pw,
		DB:       0,
	})

	if p := rds.Ping(context.Background()).Err(); p != nil {
		return nil, p
	}

	conn := Connection{
		DB:  db,
		Rds: rds,
	}

	return &conn, nil
}

type Connection struct {
	DB  *sqlx.DB
	Rds *redis.Client
}
