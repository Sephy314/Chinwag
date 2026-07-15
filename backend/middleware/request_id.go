package middleware

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/Sephy314/chinwag/shared/response"
	"github.com/labstack/echo/v5"
)

func RequestIDInjector() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) error {
			rid := c.Response().Header().Get(echo.HeaderXRequestID)
			if rid != "" {
				c.Set(response.RequestIDKey, rid)
			}
			return next(c)
		}
	}
}

func ResponseIDInjector() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) error {
			rid, _ := echo.ContextGet[string](c, response.RequestIDKey)
			if rid == "" {
				return next(c)
			}

			origWriter := c.Response()
			bufWriter := &bufferedJSONWriter{ResponseWriter: origWriter}
			c.SetResponse(bufWriter)
			defer c.SetResponse(origWriter)

			err := next(c)
			if err != nil {
				return writeErrorResponse(c, origWriter, err, rid)
			}

			if bufWriter.body.Len() == 0 {
				return nil
			}

			var data map[string]any
			if err := json.Unmarshal(bufWriter.body.Bytes(), &data); err != nil {
				origWriter.Header().Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
				origWriter.WriteHeader(bufWriter.status)
				_, _ = origWriter.Write(bufWriter.body.Bytes())
				return nil
			}

			data["request_id"] = rid

			origWriter.Header().Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
			origWriter.WriteHeader(bufWriter.status)
			return json.NewEncoder(origWriter).Encode(data)
		}
	}
}

func writeErrorResponse(_ *echo.Context, w http.ResponseWriter, err error, rid string) error {
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
	} else {
		msg = http.StatusText(code)
	}

	errResp := response.Error(msg)
	errResp.RequestID = rid

	w.Header().Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	w.WriteHeader(code)
	return json.NewEncoder(w).Encode(errResp)
}

type bufferedJSONWriter struct {
	http.ResponseWriter
	body        bytes.Buffer
	status      int
	wroteHeader bool
}

func (w *bufferedJSONWriter) WriteHeader(code int) {
	w.status = code
}

func (w *bufferedJSONWriter) Write(b []byte) (int, error) {
	h := w.Header().Get(echo.HeaderContentType)
	if h == echo.MIMEApplicationJSON {
		return w.body.Write(b)
	}
	if !w.wroteHeader {
		if w.status == 0 {
			w.status = 200
		}
		w.ResponseWriter.WriteHeader(w.status)
		w.wroteHeader = true
	}
	return w.ResponseWriter.Write(b)
}
