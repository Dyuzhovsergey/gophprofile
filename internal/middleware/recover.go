package middleware

import (
	"net/http"

	"go.uber.org/zap"
)

// Recover перехватывает panic внутри HTTP handler-ов и возвращает 500.
func Recover(log *zap.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if recovered := recover(); recovered != nil {
					log.Error(
						"panic recovered",
						zap.Any("panic", recovered),
						zap.String("method", r.Method),
						zap.String("path", r.URL.Path),
					)

					http.Error(w, "internal server error", http.StatusInternalServerError)
				}
			}()

			next.ServeHTTP(w, r)
		})
	}
}
