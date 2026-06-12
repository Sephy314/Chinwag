package errs

import (
	"errors"
	"net/http"

	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5/pgconn"
)

var ErrConflict = &AppError{
	Status:  http.StatusConflict,
	Message: "User already exists",
}

func parseDBError(err error) error {
	var pgErr *pgconn.PgError

	if !errors.As(err, &pgErr) {
		return err
	}

	switch pgErr.Code {
	case pgerrcode.UniqueViolation:
		return ErrConflict

	default:
		return err
	}
}
