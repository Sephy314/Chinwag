package errs

import (
	"errors"
	"net/http"
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
		parseDBError,
	}

	for _, parser := range listErrParsers {
		parsed := parser(err)

		if a, ok := errors.AsType[*AppError](parsed); ok {
			return a.Status, a.Message
		}
	}

	return http.StatusInternalServerError, err.Error()
}
