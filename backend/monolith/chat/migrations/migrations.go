package chatmigrations

import (
	"database/sql"
	"embed"

	"github.com/pressly/goose/v3"
)

//go:embed *.sql
var FS embed.FS

func Run(db *sql.DB) error {
	goose.SetBaseFS(FS)
	return goose.Up(db, ".")
}
