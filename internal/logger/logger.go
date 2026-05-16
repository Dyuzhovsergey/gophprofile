package logger

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"

	observabilitylogging "github.com/Dyuzhovsergey/gophprofile/internal/observability/logging"
)

// Init создаёт slog-логгер с JSON-форматом.
func Init(level string, serviceName string, environment string) (*slog.Logger, error) {
	logLevel, err := parseLevel(level)
	if err != nil {
		return nil, err
	}

	cfg := observabilitylogging.NewConfig(serviceName, environment)

	handler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: logLevel,
	})

	log := slog.New(handler).With(
		slog.String("service", cfg.ServiceName),
		slog.String("environment", cfg.Environment),
	)

	slog.SetDefault(log)

	return log, nil
}

// NewNop создаёт логгер, который ничего не пишет.
func NewNop() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// Err возвращает slog-атрибут для ошибки в едином формате.
func Err(err error) slog.Attr {
	return slog.Any("error", err)
}

// parseLevel преобразует строковый уровень логирования в slog.Level.
func parseLevel(level string) (slog.Level, error) {
	switch strings.ToLower(strings.TrimSpace(level)) {
	case "", "info":
		return slog.LevelInfo, nil
	case "debug":
		return slog.LevelDebug, nil
	case "warn", "warning":
		return slog.LevelWarn, nil
	case "error":
		return slog.LevelError, nil
	default:
		return slog.LevelInfo, fmt.Errorf("parse log level: unknown level %q", level)
	}
}
