package authmigrations

import (
	"database/sql"

	"github.com/Sephy314/chinwag/backend/services/auth/shared/logger"
	"github.com/pressly/goose/v3"
)

type gooseLogger struct {
	l logger.Logger
}

func (g *gooseLogger) Printf(msg string, args ...any) { g.l.Info(msg, args...) }
func (g *gooseLogger) Fatalf(msg string, args ...any)  { g.l.Fatal(msg, args...) }

func RunAll(dbUrl string, log logger.Logger) error {
	db, err := sql.Open("pgx", dbUrl)
	if err != nil {
		return err
	}
	defer db.Close()

	goose.SetDialect("postgres")
	goose.SetLogger(&gooseLogger{l: log})

	if err := Run(db); err != nil {
		return err
	}

	log.Info("auth service database migrations completed")
	return nil
}
