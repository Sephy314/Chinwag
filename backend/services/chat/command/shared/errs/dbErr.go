package errs

import (
	"database/sql"
	"errors"
	"net/http"

	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5/pgconn"
)

var ErrNotFound = &AppError{
	Status:  http.StatusNotFound,
	Message: "Not found",
}

func parseDBError(err error) error {
	switch {
	case errors.Is(err, sql.ErrNoRows):
		return ErrNotFound
	}

	if pgErr, ok := errors.AsType[*pgconn.PgError](err); ok {
		switch pgErr.Code {
		case pgerrcode.UniqueViolation:
			return &AppError{
				Status:  http.StatusConflict,
				Message: "Resource already exists",
			}
		}
	}

	return err
}
