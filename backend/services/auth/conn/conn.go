package conn

import (
	"context"
	"os"

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

func LoadConnectionConfig() *ConnectionConfig {
	dbUrl := os.Getenv("AUTH_DB_URL")
	if dbUrl == "" {
		dbUrl = "postgres://sephy:ouilala0328@localhost:5432/chinwag_auth?sslmode=disable"
	}

	redisAddr := os.Getenv("REDIS_ADDR")
	if redisAddr == "" {
		redisAddr = "localhost:6379"
	}

	redisPassword := os.Getenv("REDIS_PW")

	return &ConnectionConfig{
		DBUrl:         dbUrl,
		RedisAddr:     redisAddr,
		RedisPassword: redisPassword,
	}
}
