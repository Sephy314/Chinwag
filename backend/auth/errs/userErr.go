package errs

import "net/http"

var ErrInvalidCreds = &AppError{
	Status:  http.StatusBadRequest,
	Message: "Invalid Credentials",
}
