package middleware

import (
	"context"
	"net/http"
	"strings"
)

const userIDHeader = "X-User-ID"

type userIDContextKey struct{}

// RequireUserID проверяет обязательный заголовок X-User-ID и кладёт userID в context.
func RequireUserID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userID := strings.TrimSpace(r.Header.Get(userIDHeader))
		if userID == "" {
			http.Error(w, "missing X-User-ID header", http.StatusBadRequest)
			return
		}

		ctx := context.WithValue(r.Context(), userIDContextKey{}, userID)

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// UserIDFromContext возвращает userID из context.
func UserIDFromContext(ctx context.Context) (string, bool) {
	userID, ok := ctx.Value(userIDContextKey{}).(string)
	if !ok || userID == "" {
		return "", false
	}

	return userID, true
}
