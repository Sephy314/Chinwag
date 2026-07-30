package errs

import (
	"database/sql"
	"errors"
	"net/http"
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
	return err
}
