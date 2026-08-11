package main

import (
	"log/slog"
	"os"
	"strings"
)

// newJSONHandler builds a JSON slog handler whose minimum level is controlled
// by the LOG_LEVEL environment variable ("debug"/"info"/"warn"/"error"). When
// LOG_LEVEL is unset the level defaults to INFO, so DEBUG logs (DB/Redis/NATS
// tracing) are suppressed unless explicitly enabled.
func newJSONHandler() slog.Handler {
	level := slog.LevelInfo
	if v, ok := os.LookupEnv("LOG_LEVEL"); ok {
		switch strings.ToLower(v) {
		case "debug":
			level = slog.LevelDebug
		case "warn":
			level = slog.LevelWarn
		case "error":
			level = slog.LevelError
		}
	}
	return slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level})
}
