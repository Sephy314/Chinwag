package conn

import (
	"context"
	"log/slog"

	"github.com/Sephy314/chinwag/backend/services/chat/command/nats"
	"github.com/jmoiron/sqlx"
	"github.com/redis/go-redis/v9"
)

import _ "github.com/jackc/pgx/v5/stdlib"

type Connection struct {
	DB   *sqlx.DB
	Rds  *redis.Client
	Nats *nats.JetStreamEventPublisher
}

type ConnectionConfig struct {
	DBUrl         string
	RedisAddr     string
	RedisPassword string
	NatsURL       string
	Log           *slog.Logger
}

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

	conn := &Connection{
		DB:  db,
		Rds: rds,
	}

	if cfg.NatsURL != "" {
		natsPub, err := nats.NewJetStreamEventPublisher(context.Background(), cfg.NatsURL, cfg.Log)
		if err != nil {
			return nil, err
		}
		conn.Nats = natsPub
	}

	return conn, nil
}

func (c *Connection) Close() {
	if c.DB != nil {
		c.DB.Close()
	}
	if c.Nats != nil {
		c.Nats.Close()
	}
}
