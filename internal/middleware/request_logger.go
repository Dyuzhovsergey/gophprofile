package middleware

import (
	"net/http"
	"time"

	"go.uber.org/zap"
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
func RequestLogger(log *zap.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			startedAt := time.Now()

			wrappedWriter := &responseWriter{
				ResponseWriter: w,
				statusCode:     http.StatusOK,
			}

			next.ServeHTTP(wrappedWriter, r)

			log.Info(
				"HTTP request completed",
				zap.String("method", r.Method),
				zap.String("path", r.URL.Path),
				zap.Int("status", wrappedWriter.statusCode),
				zap.Duration("duration", time.Since(startedAt)),
			)
		})
	}
}
