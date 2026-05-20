package metrics

import (
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// HTTPRequestsTotal считает общее количество HTTP-запросов.
	HTTPRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: Namespace,
			Name:      "http_requests_total",
			Help:      "Total number of HTTP requests.",
		},
		[]string{"method", "route", "status"},
	)

	// HTTPRequestDurationSeconds измеряет длительность HTTP-запросов.
	HTTPRequestDurationSeconds = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: Namespace,
			Name:      "http_request_duration_seconds",
			Help:      "HTTP request duration in seconds.",
			Buckets:   prometheus.DefBuckets,
		},
		[]string{"method", "route", "status"},
	)

	// HTTPRequestsInFlight показывает количество HTTP-запросов, которые сейчас обрабатываются.
	HTTPRequestsInFlight = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: Namespace,
			Name:      "http_requests_in_flight",
			Help:      "Number of HTTP requests currently being processed.",
		},
		[]string{"method", "route"},
	)

	// HTTPResponseSizeBytes измеряет размер HTTP-ответов в байтах.
	HTTPResponseSizeBytes = promauto.NewHistogramVec(
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
	)
)

// ObserveHTTPRequest записывает метрики завершённого HTTP-запроса.
func ObserveHTTPRequest(method string, route string, status int, duration time.Duration, responseSizeBytes int) {
	statusText := strconv.Itoa(status)

	HTTPRequestsTotal.WithLabelValues(method, route, statusText).Inc()
	HTTPRequestDurationSeconds.WithLabelValues(method, route, statusText).Observe(duration.Seconds())
	HTTPResponseSizeBytes.WithLabelValues(method, route, statusText).Observe(float64(responseSizeBytes))
}

// IncHTTPRequestsInFlight увеличивает количество активных HTTP-запросов.
func IncHTTPRequestsInFlight(method string, route string) {
	HTTPRequestsInFlight.WithLabelValues(method, route).Inc()
}

// DecHTTPRequestsInFlight уменьшает количество активных HTTP-запросов.
func DecHTTPRequestsInFlight(method string, route string) {
	HTTPRequestsInFlight.WithLabelValues(method, route).Dec()
}
