package errs

import "net/http"

var (
	InvalidAlgErr = &AppError{
		Status:  http.StatusBadRequest,
		Message: "Algorithm invalid",
	}

	InvalidTokenErr = &AppError{
		Status:  http.StatusBadRequest,
		Message: "Invalid token",
	}

	ErrNoKey = &AppError{
		Status:  http.StatusBadRequest,
		Message: "No key",
	}
)
