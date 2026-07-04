// Package logger provides structured logging configuration for the sandbox API.
package logger

import (
	"log/slog"
	"os"
)

// New returns a structured JSON logger.
func New() *slog.Logger {
	return slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
}
