package logger

import (
	"log/slog"
	"os"
	"strings"
)

type Logger interface {
	Info(msg string, args ...any)
	Error(msg string, args ...any)
	Debug(msg string, args ...any)
	Warn(msg string, args ...any)
	Fatal(msg string, args ...any)
	With(args ...any) Logger
}

type slogLogger struct {
	l *slog.Logger
}

func New() Logger {
	return NewWith(slog.New(NewHandler()))
}

// NewHandler builds a JSON handler whose minimum level is controlled by the
// LOG_LEVEL environment variable ("debug"/"info"/"warn"/"error"). When
// LOG_LEVEL is unset the level defaults to INFO, so DEBUG logs (DB/Redis/NATS
// tracing) are suppressed unless explicitly enabled.
func NewHandler() slog.Handler {
	return handlerForLevel(parseLogLevel())
}

func parseLogLevel() slog.Level {
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
	return level
}

func handlerForLevel(level slog.Level) slog.Handler {
	return slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level})
}

// NewWith wraps an existing *slog.Logger so callers can share a single handler
// across the application logger and infra-level loggers (DB/Redis tracing).
func NewWith(l *slog.Logger) Logger {
	return &slogLogger{l: l}
}

func (l *slogLogger) Info(msg string, args ...any)  { l.l.Info(msg, args...) }
func (l *slogLogger) Error(msg string, args ...any) { l.l.Error(msg, args...) }
func (l *slogLogger) Debug(msg string, args ...any) { l.l.Debug(msg, args...) }
func (l *slogLogger) Warn(msg string, args ...any)  { l.l.Warn(msg, args...) }

func (l *slogLogger) Fatal(msg string, args ...any) {
	l.l.Error(msg, args...)
	os.Exit(1)
}

func (l *slogLogger) With(args ...any) Logger {
	return &slogLogger{l: l.l.With(args...)}
}
