package logger

import (
	"log/slog"
	"os"

	"github.com/google/uuid"
)

const TRACE_ID = "TRACE_ID"

func GetLoggerWithTID(id string) *slog.Logger {
	if id == "" {
		return slog.With(TRACE_ID, uuid.NewString())
	}

	return slog.With(TRACE_ID, id)
}

func SetJsonAsDefault(level slog.Level) {
	handler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: level,
	})

	logger := slog.New(handler)
	slog.SetDefault(logger)
}

func SetTextAsDefault(level slog.Level) {
	handler := slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: level,
	})

	logger := slog.New(handler)
	slog.SetDefault(logger)
}
