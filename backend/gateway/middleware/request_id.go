package middleware

import (
	"github.com/google/uuid"
	"github.com/labstack/echo/v5"
)

const HeaderXRequestID = "X-Request-ID"
const ContextKeyRequestID = "request_id"

func RequestID() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) error {
			rid := c.Request().Header.Get(HeaderXRequestID)
			if rid == "" {
				rid = uuid.New().String()
			}

			c.Request().Header.Set(HeaderXRequestID, rid)
			c.Set(ContextKeyRequestID, rid)

			return next(c)
		}
	}
}
