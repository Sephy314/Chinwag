// @title Chinwag API
// @version 1.0
// @description Chinwag Chat Application API
// @host localhost:8080
// @BasePath /
// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
package main

import (
	"database/sql"
	"os"

	chatmigrations "github.com/Sephy314/chinwag/backend/monolith/chat/migrations"
	roommigrations "github.com/Sephy314/chinwag/backend/monolith/room/migrations"
	"github.com/Sephy314/chinwag/backend/monolith/router"
	"github.com/Sephy314/chinwag/backend/monolith/shared/logger"
	"github.com/joho/godotenv"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
)

func main() {
	_ = godotenv.Load()

	log := logger.New()

	if err := migrate(log); err != nil {
		log.Fatal("failed to run migrations", "error", err)
	}

	e, err := router.SetUpRouter(log)
	if err != nil {
		log.Fatal("failed to setup router", "error", err)
	}
	err = e.Start("0.0.0.0:8080")
	if err != nil {
		log.Fatal("server failed to start", "error", err)
	}
}

type gooseLogger struct {
	l logger.Logger
}

func (g *gooseLogger) Printf(msg string, args ...any) { g.l.Info(msg, args...) }
func (g *gooseLogger) Fatalf(msg string, args ...any)  { g.l.Fatal(msg, args...) }

func migrate(log logger.Logger) error {
	dsn := os.Getenv("DB_DSN_CHINWAG")
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return err
	}
	defer db.Close()

	goose.SetDialect("postgres")
	goose.SetLogger(&gooseLogger{l: log})

	for _, fn := range []func(*sql.DB) error{
		roommigrations.Run,
		chatmigrations.Run,
	} {
		if err := fn(db); err != nil {
			return err
		}
	}

	log.Info("database migrations completed")
	return nil
}
