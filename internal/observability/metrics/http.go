package metrics

import (
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

// HTTPMetrics содержит Prometheus-метрики HTTP-слоя.
type HTTPMetrics struct {
	requestsTotal          *prometheus.CounterVec
	requestDurationSeconds *prometheus.HistogramVec
	requestsInFlight       *prometheus.GaugeVec
	responseSizeBytes      *prometheus.HistogramVec
}

// NewHTTPMetrics создаёт и регистрирует HTTP-метрики.
func NewHTTPMetrics(registry prometheus.Registerer) *HTTPMetrics {
	m := &HTTPMetrics{
		requestsTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: Namespace,
				Name:      "http_requests_total",
				Help:      "Total number of HTTP requests.",
			},
			[]string{"method", "route", "status"},
		),

		requestDurationSeconds: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: Namespace,
				Name:      "http_request_duration_seconds",
				Help:      "HTTP request duration in seconds.",
				Buckets:   prometheus.DefBuckets,
			},
			[]string{"method", "route", "status"},
		),

		requestsInFlight: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: Namespace,
				Name:      "http_requests_in_flight",
				Help:      "Number of HTTP requests currently being processed.",
			},
			[]string{"method", "route"},
		),

		responseSizeBytes: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: Namespace,
				Name:      "http_response_size_bytes",
				Help:      "HTTP response size in bytes.",
				Buckets: []float64{
					100,
					1_000,
					10_000,
					100_000,
					1_000_000,
					10_000_000,
				},
			},
			[]string{"method", "route", "status"},
		),
	}

	registry.MustRegister(
		m.requestsTotal,
		m.requestDurationSeconds,
		m.requestsInFlight,
		m.responseSizeBytes,
	)

	return m
}

// ObserveHTTPRequest записывает метрики завершённого HTTP-запроса.
func (m *HTTPMetrics) ObserveHTTPRequest(
	method string,
	route string,
	status int,
	duration time.Duration,
	responseSizeBytes int,
) {
	if m == nil {
		return
	}

	statusText := strconv.Itoa(status)

	m.requestsTotal.WithLabelValues(method, route, statusText).Inc()
	m.requestDurationSeconds.WithLabelValues(method, route, statusText).Observe(duration.Seconds())
	m.responseSizeBytes.WithLabelValues(method, route, statusText).Observe(float64(responseSizeBytes))
}

// IncHTTPRequestsInFlight увеличивает количество активных HTTP-запросов.
func (m *HTTPMetrics) IncHTTPRequestsInFlight(method string, route string) {
	if m == nil {
		return
	}

	m.requestsInFlight.WithLabelValues(method, route).Inc()
}

// DecHTTPRequestsInFlight уменьшает количество активных HTTP-запросов.
func (m *HTTPMetrics) DecHTTPRequestsInFlight(method string, route string) {
	if m == nil {
		return
	}

	m.requestsInFlight.WithLabelValues(method, route).Dec()
}
