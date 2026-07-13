package errs

import (
	"database/sql"
	"errors"
	"net/http"

	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5/pgconn"
)

var ErrConflict = &AppError{
	Status:  http.StatusConflict,
	Message: "User already exists",
}

var ErrNotFound = &AppError{
	Status:  http.StatusNotFound,
	Message: "User not found",
}

var ErrCacheNotFound = &AppError{
	Status:  http.StatusNotFound,
	Message: "Cached data not found",
}

func parseDBError(err error) error {
	switch {
	case errors.Is(err, sql.ErrNoRows):
		return ErrNotFound
	}

	if pgErr, ok := errors.AsType[*pgconn.PgError](err); ok {
		switch pgErr.Code {
		case pgerrcode.UniqueViolation:
			return ErrConflict
		}
	}

	return err
}
