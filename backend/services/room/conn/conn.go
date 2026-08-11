package conn

import (
	"context"
	"log/slog"

	"github.com/jmoiron/sqlx"
	"github.com/redis/go-redis/v9"
)

func NewConnection(cfg *ConnectionConfig) (*Connection, error) {
	db, err := newDB(cfg.DBUrl, cfg.Log)
	if err != nil {
		return nil, err
	}

	rds := redis.NewClient(&redis.Options{
		Addr:     cfg.RedisAddr,
		Password: cfg.RedisPassword,
		DB:       0,
	})
	if cfg.Log != nil {
		rds.AddHook(&redisLogHook{log: cfg.Log})
	}

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
	Log           *slog.Logger
}
