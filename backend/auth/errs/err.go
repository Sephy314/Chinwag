package errs

import (
	"errors"
	"net/http"

	"golang.org/x/crypto/bcrypt"
)

type AppError struct {
	Status  int    `json:"status"`
	Message string `json:"message"`
}

func (e *AppError) Error() string {
	return e.Message
}

func ParseError(err error) (int, string) {
	listErrParsers := []func(error) error{
		parseAuthError,
		parseDBError,
	}

	for _, parser := range listErrParsers {
		parsed := parser(err)

		if a, ok := errors.AsType[*AppError](parsed); ok {
			return a.Status, a.Message
		}
	}

	// For any other error, return a generic AppError to avoid leaking internal details
	return http.StatusInternalServerError, "Internal Server Error"
}

func parseAuthError(err error) error {
	if errors.Is(err, bcrypt.ErrMismatchedHashAndPassword) {
		return &AppError{
			Status:  http.StatusBadRequest,
			Message: "Invalid Creds",
		}
	}

	return err
}
