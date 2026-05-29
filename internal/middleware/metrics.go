package middleware

import (
	"net/http"
	"strings"
	"time"

	observabilitymetrics "github.com/Dyuzhovsergey/gophprofile/internal/observability/metrics"
	"github.com/go-chi/chi/v5"
)

// metricResponseWriter сохраняет HTTP-статус и размер ответа для метрик.
type metricResponseWriter struct {
	http.ResponseWriter
	statusCode   int
	bytesWritten int
}

// WriteHeader сохраняет HTTP-статус ответа.
func (w *metricResponseWriter) WriteHeader(statusCode int) {
	if w.statusCode != 0 {
		return
	}

	w.statusCode = statusCode
	w.ResponseWriter.WriteHeader(statusCode)
}

// Write записывает тело ответа и считает количество записанных байт.
func (w *metricResponseWriter) Write(data []byte) (int, error) {
	if w.statusCode == 0 {
		w.statusCode = http.StatusOK
	}

	written, err := w.ResponseWriter.Write(data)
	w.bytesWritten += written

	return written, err
}

// HTTPMetrics собирает Prometheus-метрики по HTTP-запросам.
func HTTPMetrics(metrics *observabilitymetrics.HTTPMetrics) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			startedAt := time.Now()

			wrappedWriter := &metricResponseWriter{
				ResponseWriter: w,
			}

			routeForInFlight := routePattern(r)
			metrics.IncHTTPRequestsInFlight(r.Method, routeForInFlight)
			defer metrics.DecHTTPRequestsInFlight(r.Method, routeForInFlight)

			next.ServeHTTP(wrappedWriter, r)

			if wrappedWriter.statusCode == 0 {
				wrappedWriter.statusCode = http.StatusOK
			}

			route := routePattern(r)

			metrics.ObserveHTTPRequest(
				r.Method,
				route,
				wrappedWriter.statusCode,
				time.Since(startedAt),
				wrappedWriter.bytesWritten,
			)
		})
	}
}

// routePattern возвращает шаблон chi-маршрута для Prometheus labels.
func routePattern(r *http.Request) string {
	routeContext := chi.RouteContext(r.Context())
	if routeContext == nil {
		return "unknown"
	}

	route := strings.TrimSpace(routeContext.RoutePattern())
	if route != "" {
		return route
	}

	return "unknown"
}
