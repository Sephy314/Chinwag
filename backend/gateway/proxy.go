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

		if req.Header.Get("X-Forwarded-Proto") == "" {
			proto := "http"
			if req.TLS != nil {
				proto = "https"
			}
			req.Header.Set("X-Forwarded-Proto", proto)
		}
		if req.Header.Get("X-Forwarded-Host") == "" {
			req.Header.Set("X-Forwarded-Host", req.Host)
		}

		req.Host = target.Host
		if stripPrefix {
			req.Header.Set("X-Forwarded-Prefix", prefix)
			req.URL.Path = strings.TrimPrefix(req.URL.Path, prefix)
			if req.URL.Path == "" {
				req.URL.Path = "/"
			}
		} else {
			// The ingress stripPrefix middleware set X-Forwarded-Prefix=/api.
			// For routes we don't strip, that prefix would leak into the
			// backend's DPoP htu reconstruction (RequestHTU) and make it
			// disagree with the htu the frontend signs (which strips /api).
			// Drop it so the backend reconstructs the plain path.
			req.Header.Del("X-Forwarded-Prefix")
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
