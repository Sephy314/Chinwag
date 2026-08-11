package logger

import (
	"context"
	"log/slog"
	"os"
	"testing"
)

func restoreEnv(t *testing.T, old string, had bool) {
	t.Helper()
	t.Cleanup(func() {
		if had {
			os.Setenv("LOG_LEVEL", old)
		} else {
			os.Unsetenv("LOG_LEVEL")
		}
	})
}

func TestParseLogLevel_DefaultIsInfo(t *testing.T) {
	old, had := os.LookupEnv("LOG_LEVEL")
	restoreEnv(t, old, had)
	os.Unsetenv("LOG_LEVEL")

	if got := parseLogLevel(); got != slog.LevelInfo {
		t.Fatalf("unset LOG_LEVEL -> level %v, want info", got)
	}
}

func TestParseLogLevel_Debug(t *testing.T) {
	old, had := os.LookupEnv("LOG_LEVEL")
	restoreEnv(t, old, had)
	os.Setenv("LOG_LEVEL", "debug")

	if got := parseLogLevel(); got != slog.LevelDebug {
		t.Fatalf("LOG_LEVEL=debug -> level %v, want debug", got)
	}
}

func TestParseLogLevel_UnknownFallsBackToInfo(t *testing.T) {
	old, had := os.LookupEnv("LOG_LEVEL")
	restoreEnv(t, old, had)
	os.Setenv("LOG_LEVEL", "verbose")

	if got := parseLogLevel(); got != slog.LevelInfo {
		t.Fatalf("LOG_LEVEL=verbose -> level %v, want info", got)
	}
}

func TestHandlerLevel_DebugSuppressedAtDefault(t *testing.T) {
	if h := handlerForLevel(slog.LevelInfo); h.Enabled(context.Background(), slog.LevelDebug) {
		t.Fatal("DEBUG should be suppressed at default (info) level")
	}
}

func TestHandlerLevel_DebugEnabled(t *testing.T) {
	if h := handlerForLevel(slog.LevelDebug); !h.Enabled(context.Background(), slog.LevelDebug) {
		t.Fatal("DEBUG should be enabled at debug level")
	}
}
