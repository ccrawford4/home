// Package logger provides structured logging configuration for the sandbox CLI.
package logger

import (
	"log/slog"
	"os"
)

// New returns a structured JSON logger.
func New() *slog.Logger {
	return slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
}
