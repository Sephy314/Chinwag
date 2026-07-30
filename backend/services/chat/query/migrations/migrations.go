package chatquerymigrations

import (
	"database/sql"
	"embed"
	"fmt"
	"log/slog"

	"github.com/pressly/goose/v3"
)

//go:embed *.sql
var FS embed.FS

type slogGoose struct {
	l *slog.Logger
}

func (g *slogGoose) Printf(format string, v ...any) {
	g.l.Info(fmt.Sprintf(format, v...))
}

func (g *slogGoose) Fatalf(format string, v ...any) {
	g.l.Error(fmt.Sprintf(format, v...))
}

func Run(db *sql.DB) error {
	goose.SetBaseFS(FS)
	return goose.Up(db, ".")
}

func RunAll(dbUrl string, log *slog.Logger) error {
	db, err := sql.Open("pgx", dbUrl)
	if err != nil {
		return fmt.Errorf("failed to open database for migrations: %w", err)
	}
	defer db.Close()

	goose.SetDialect("postgres")
	goose.SetLogger(&slogGoose{l: log})

	if err := Run(db); err != nil {
		return fmt.Errorf("chat query migrations failed: %w", err)
	}

	log.Info("chat query database migrations completed")
	return nil
}
