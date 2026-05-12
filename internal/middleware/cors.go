package middleware

import "net/http"

const (
	corsAllowOrigin  = "*"
	corsAllowMethods = "GET, POST, DELETE, OPTIONS"
	corsAllowHeaders = "Content-Type, X-User-ID"
	corsMaxAge       = "86400"
)

// CORS добавляет HTTP-заголовки для кросс-доменных запросов из браузера.
func CORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin == "" {
			next.ServeHTTP(w, r)
			return
		}

		header := w.Header()
		header.Set("Access-Control-Allow-Origin", corsAllowOrigin)
		header.Set("Access-Control-Allow-Methods", corsAllowMethods)
		header.Set("Access-Control-Allow-Headers", corsAllowHeaders)
		header.Set("Access-Control-Max-Age", corsMaxAge)

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}