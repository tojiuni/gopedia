package logging

import (
	"log/slog"
	"os"
	"strings"
)

// Logger is a lightweight wrapper around slog.Logger for shared logging helpers.
type Logger struct {
	base *slog.Logger
}

// New creates a logger wrapper from the provided slog logger.
func New(base *slog.Logger) *Logger {
	if base == nil {
		base = slog.Default()
	}
	return &Logger{base: base}
}

// Default returns a wrapper over the process default slog logger.
func Default() *Logger {
	return New(slog.Default())
}

// Base returns the underlying slog logger.
func (l *Logger) Base() *slog.Logger {
	return l.base
}

// Info logs at info level.
func (l *Logger) Info(msg string, args ...any) {
	l.base.Info(msg, args...)
}

// Warn logs at warn level.
func (l *Logger) Warn(msg string, args ...any) {
	l.base.Warn(msg, args...)
}

// Debug logs at debug level.
func (l *Logger) Debug(msg string, args ...any) {
	l.base.Debug(msg, args...)
}

// NewDefaultSlog builds a default text slog logger using GOPEDIA_LOG_LEVEL.
func NewDefaultSlog() *slog.Logger {
	level := parseLevel(os.Getenv("GOPEDIA_LOG_LEVEL"))
	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level}))
}

func parseLevel(raw string) slog.Level {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
