package handlers

import (
	"net/http"

	"github.com/Dyuzhovsergey/gophprofile/internal/resilience/circuitbreaker"
)

const externalDependencyUnavailableDetails = "External dependency is temporarily unavailable"

// isExternalDependencyUnavailable проверяет,
// что внешний вызов отклонён Circuit Breaker-ом.
func isExternalDependencyUnavailable(err error) bool {
	return circuitbreaker.IsUnavailable(err)
}

// writeExternalDependencyUnavailable записывает HTTP 503,
// когда Circuit Breaker временно блокирует внешнюю зависимость.
func writeExternalDependencyUnavailable(w http.ResponseWriter) {
	writeJSONError(
		w,
		http.StatusServiceUnavailable,
		ErrorResponse{
			Error:   "Service unavailable",
			Details: externalDependencyUnavailableDetails,
		},
	)
}
