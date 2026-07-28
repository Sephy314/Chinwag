package roommigrations

import (
	"database/sql"
	"embed"
	"fmt"

	"github.com/pressly/goose/v3"
)

//go:embed *.sql
var FS embed.FS

func Run(db *sql.DB) error {
	goose.SetBaseFS(FS)
	return goose.Up(db, ".")
}

func RunAll(dbUrl string, log interface{ Info(string, ...any) }) error {
	db, err := sql.Open("pgx", dbUrl)
	if err != nil {
		return fmt.Errorf("failed to open database for migrations: %w", err)
	}
	defer db.Close()

	goose.SetDialect("postgres")

	if err := Run(db); err != nil {
		return fmt.Errorf("room migrations failed: %w", err)
	}

	log.Info("room database migrations completed")
	return nil
}
