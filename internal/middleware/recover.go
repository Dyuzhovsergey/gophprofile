package middleware

import (
	"log/slog"
	"net/http"

	"github.com/Dyuzhovsergey/gophprofile/internal/logger"
)

// Recover перехватывает panic внутри HTTP handler-ов и возвращает 500.
func Recover(log *slog.Logger) func(http.Handler) http.Handler {
	if log == nil {
		log = logger.NewNop()
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if recovered := recover(); recovered != nil {
					log.Error(
						"panic recovered",
						slog.Any("panic", recovered),
						slog.String("method", r.Method),
						slog.String("path", r.URL.Path),
					)

					http.Error(w, "internal server error", http.StatusInternalServerError)
				}
			}()

			next.ServeHTTP(w, r)
		})
	}
}
