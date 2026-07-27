package logger

import (
	"log/slog"
	"os"
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
	return &slogLogger{
		l: slog.New(slog.NewJSONHandler(os.Stdout, nil)),
	}
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
