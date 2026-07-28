package main

import (
	"net/http"
	"net/http/httputil"
	"net/url"
	"slices"
	"strings"

	"github.com/labstack/echo/v5"
)

func proxyHandler(targetURL string, stripPrefix bool, prefix string) echo.HandlerFunc {
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

	return func(c *echo.Context) error {
		proxy.ServeHTTP(c.Response(), c.Request())
		return nil
	}
}

func setupRoutes(e *echo.Echo, cfg *Config) {
	for prefix, targetURL := range cfg.Services {
		shouldStrip := slices.Contains(cfg.StripPrefix, prefix)
		handler := proxyHandler(targetURL, shouldStrip, prefix)
		e.Any(prefix+"/*", handler)
	}

	if cfg.Default != "" {
		handler := proxyHandler(cfg.Default, false, "")
		e.Any("/*", handler)
	}
}
