package middleware

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/riandyrn/otelchi"
)

// Tracing создаёт middleware для трассировки входящих HTTP-запросов.
//
// routes нужен otelchi, чтобы называть spans по шаблону маршрута,
// например GET /api/v1/avatars/{avatar_id}, а не по конкретному URL.
func Tracing(serviceName string, routes chi.Routes) func(http.Handler) http.Handler {
	return otelchi.Middleware(
		serviceName,
		otelchi.WithChiRoutes(routes),
		otelchi.WithRequestMethodInSpanName(true),
		otelchi.WithTraceResponseHeaders(otelchi.TraceHeaderConfig{
			TraceIDHeader:      "X-Trace-ID",
			TraceSampledHeader: "X-Trace-Sampled",
		}),
	)
}