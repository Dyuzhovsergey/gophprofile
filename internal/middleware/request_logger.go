package middleware

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/Dyuzhovsergey/gophprofile/internal/logger"
	observabilitylogging "github.com/Dyuzhovsergey/gophprofile/internal/observability/logging"
)

// responseWriter хранит HTTP-статус ответа для логирования.
type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

// WriteHeader сохраняет статус ответа и передаёт его дальше.
func (w *responseWriter) WriteHeader(statusCode int) {
	w.statusCode = statusCode
	w.ResponseWriter.WriteHeader(statusCode)
}

// RequestLogger логирует информацию о каждом HTTP-запросе.
func RequestLogger(log *slog.Logger) func(http.Handler) http.Handler {
	if log == nil {
		log = logger.NewNop()
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			startedAt := time.Now()

			wrappedWriter := &responseWriter{
				ResponseWriter: w,
				statusCode:     http.StatusOK,
			}

			next.ServeHTTP(wrappedWriter, r)

			log.LogAttrs(
				r.Context(),
				slog.LevelInfo,
				"HTTP request completed",
				observabilitylogging.AppendTraceAttrs(
					r.Context(),
					slog.String("method", r.Method),
					slog.String("path", r.URL.Path),
					slog.Int("status", wrappedWriter.statusCode),
					slog.Duration("duration", time.Since(startedAt)),
				)...,
			)
		})
	}
}
