package errs

import (
	"errors"
	"net/http"

	"github.com/Sephy314/chinwag/shared/response"
	"github.com/labstack/echo/v5"
	"golang.org/x/crypto/bcrypt"
)

type AppError struct {
	Status  int    `json:"status"`
	Message string `json:"message"`
}

func (e *AppError) Error() string {
	return e.Message
}

func ParseError(err error) (int, *response.Response[any]) {
	if he, ok := errors.AsType[*echo.HTTPError](err); ok {
		msg := he.Message
		if msg == "" {
			msg = http.StatusText(he.Code)
		}
		return he.Code, response.Error(msg)
	}

	if code := echo.StatusCode(err); code != 0 {
		return code, response.Error(http.StatusText(code))
	}

	listErrParsers := []func(error) error{
		parseAuthError,
		parseDBError,
	}

	for _, parser := range listErrParsers {
		parsed := parser(err)

		if a, ok := errors.AsType[*AppError](parsed); ok {
			return a.Status, response.Error(a.Message)
		}
	}

	return http.StatusInternalServerError, response.Error("Internal Server Error")
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
