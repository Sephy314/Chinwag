package conn

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/jmoiron/sqlx"
	natslib "github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/redis/go-redis/v9"
)

import _ "github.com/jackc/pgx/v5/stdlib"

type Connection struct {
	DB  *sqlx.DB
	Rds *redis.Client
	Nc  *natslib.Conn
	Js  jetstream.JetStream
}

type ConnectionConfig struct {
	DBUrl         string
	RedisAddr     string
	RedisPassword string
	NatsURL       string
	NatsName      string
	Log           *slog.Logger
}

func NewConnection(cfg *ConnectionConfig) (*Connection, error) {
	db, err := newDB(cfg.DBUrl, cfg.Log)
	if err != nil {
		return nil, fmt.Errorf("db connect: %w", err)
	}

	conn := &Connection{DB: db}

	rds := redis.NewClient(&redis.Options{
		Addr:     cfg.RedisAddr,
		Password: cfg.RedisPassword,
		DB:       0,
	})
	if cfg.Log != nil {
		rds.AddHook(&redisLogHook{log: cfg.Log})
	}

	if err := rds.Ping(context.Background()).Err(); err != nil {
		return nil, fmt.Errorf("redis ping: %w", err)
	}

	conn.Rds = rds

	if cfg.NatsURL != "" {
		nc, err := natslib.Connect(cfg.NatsURL,
			natslib.Name(cfg.NatsName),
			natslib.ReconnectWait(2*time.Second),
			natslib.MaxReconnects(-1),
		)
		if err != nil {
			return nil, fmt.Errorf("nats connect: %w", err)
		}

		js, err := jetstream.New(nc)
		if err != nil {
			nc.Close()
			return nil, fmt.Errorf("jetstream new: %w", err)
		}

		_, err = js.CreateOrUpdateConsumer(context.Background(), "CHAT_EVENTS", jetstream.ConsumerConfig{
			Name:          "chat-projection",
			Description:   "Projection consumer for CQRS query service",
			FilterSubject: "chat.room.>",
			DeliverPolicy: jetstream.DeliverNewPolicy,
			AckPolicy:     jetstream.AckExplicitPolicy,
			MaxDeliver:    3,
		})
		if err != nil {
			nc.Close()
			return nil, fmt.Errorf("jetstream consumer setup: %w", err)
		}

		conn.Nc = nc
		conn.Js = js
	}

	return conn, nil
}

func (c *Connection) Close() {
	if c.DB != nil {
		c.DB.Close()
	}
	if c.Rds != nil {
		c.Rds.Close()
	}
	if c.Nc != nil {
		c.Nc.Close()
	}
}
