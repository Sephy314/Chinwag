package conn

import (
	"context"

	"github.com/jmoiron/sqlx"
	"github.com/redis/go-redis/v9"
)

import _ "github.com/jackc/pgx/v5/stdlib"

func NewConnection(cfg *ConnectionConfig) (*Connection, error) {
	db, err := sqlx.Connect("pgx", cfg.DBUrl)
	if err != nil {
		return nil, err
	}

	rds := redis.NewClient(&redis.Options{
		Addr:     cfg.RedisAddr,
		Password: cfg.RedisPassword,
		DB:       0,
	})

	if err := rds.Ping(context.Background()).Err(); err != nil {
		return nil, err
	}

	return &Connection{
		DB:  db,
		Rds: rds,
	}, nil
}

type Connection struct {
	DB  *sqlx.DB
	Rds *redis.Client
}

type ConnectionConfig struct {
	DBUrl         string
	RedisAddr     string
	RedisPassword string
}
