package middleware

import (
	"errors"
	"net/http"

	"github.com/Sephy314/chinwag/shared/errs"
	"github.com/Sephy314/chinwag/shared/logger"
	"github.com/Sephy314/chinwag/shared/response"
	"github.com/labstack/echo/v5"
)

func GlobalErrorHandler(log logger.Logger) func(c *echo.Context, err error) {
	return func(c *echo.Context, err error) {
		if r, rErr := echo.UnwrapResponse(c.Response()); rErr == nil && r.Committed {
			return
		}

		code := http.StatusInternalServerError
		var msg string

		var sc echo.HTTPStatusCoder
		if errors.As(err, &sc) {
			if tmp := sc.StatusCode(); tmp != 0 {
				code = tmp
			}
		}

		if he, ok := errors.AsType[*echo.HTTPError](err); ok {
			msg = he.Message
			if msg == "" {
				msg = http.StatusText(code)
			}
		} else if appErr, ok := errors.AsType[*errs.AppError](err); ok {
			code = appErr.Status
			msg = appErr.Message
		} else {
			msg = http.StatusText(code)
		}

		log.Error("http error", "status", code, "message", msg)

		_ = c.JSON(code, response.Error(msg))
	}
}
