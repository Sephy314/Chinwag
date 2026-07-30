package main

import (
	"net/http"
	"net/http/httputil"
	"net/url"
	"slices"
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
		w.WriteHeader(http.StatusBadGateway)
		w.Write([]byte(`{"success":false,"code":502,"message":"bad gateway"}`))
	}

	return proxy
}

func setupRoutes(e *echo.Echo, cfg *Config) {
	e.Pre(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) error {
			path := c.Request().URL.Path
			for prefix, targetURL := range cfg.Services {
				if strings.HasPrefix(path, prefix) {
					shouldStrip := slices.Contains(cfg.StripPrefix, prefix)
					proxy := newReverseProxy(targetURL, shouldStrip, prefix)
					proxy.ServeHTTP(c.Response(), c.Request())
					return nil
				}
			}
			return next(c)
		}
	})

}
