package slogger

import (
	"log/slog"
	"os"
)

type Logger struct {
}

func New() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
}