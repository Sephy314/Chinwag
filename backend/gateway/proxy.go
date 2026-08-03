package main

import (
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"

	"github.com/labstack/echo/v5"
)

func newReverseProxy(targetURL string, stripPrefix bool, prefix string) *httputil.ReverseProxy {
	target, err := url.Parse(targetURL)
	if err != nil {
		panic("invalid target URL: " + targetURL)
	}

	proxy := httputil.NewSingleHostReverseProxy(target)

	originalDirector := proxy.Director
	proxy.Director = func(req *http.Request) {
		originalDirector(req)
		req.Host = target.Host
		if stripPrefix {
			req.URL.Path = strings.TrimPrefix(req.URL.Path, prefix)
			if req.URL.Path == "" {
				req.URL.Path = "/"
			}
		}
	}

	proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		w.Header().Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		w.WriteHeader(http.StatusServiceUnavailable)
		w.Write([]byte(`{"success":false,"code":503,"message":"service unavailable"}`))
	}

	return proxy
}

func setupRoutes(e *echo.Echo, cfg *Config) {
	e.Pre(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) error {
			path := c.Request().URL.Path
			method := c.Request().Method

			for _, route := range cfg.Routes {
				if !strings.HasPrefix(path, route.Prefix) {
					continue
				}
				if route.Suffix != "" && !strings.HasSuffix(path, route.Suffix) {
					continue
				}
				if len(route.Methods) > 0 {
					methodMatch := false
					for _, m := range route.Methods {
						if m == method {
							methodMatch = true
							break
						}
					}
					if !methodMatch {
						continue
					}
				}
				proxy := newReverseProxy(route.TargetURL, route.StripPrefix, route.Prefix)
				proxy.ServeHTTP(c.Response(), c.Request())
				return nil
			}

			return next(c)
		}
	})

}
