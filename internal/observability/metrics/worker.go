package metrics

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

// WorkerMetrics содержит Prometheus-метрики worker-а.
type WorkerMetrics struct {
	messagesConsumedTotal     *prometheus.CounterVec
	messagesFailedTotal       *prometheus.CounterVec
	processingDurationSeconds *prometheus.HistogramVec
	retriesTotal              *prometheus.CounterVec
	activeJobs                *prometheus.GaugeVec
}

// NewWorkerMetrics создаёт и регистрирует метрики worker-а.
func NewWorkerMetrics(registry prometheus.Registerer) *WorkerMetrics {
	m := &WorkerMetrics{
		messagesConsumedTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: Namespace,
				Name:      "worker_messages_consumed_total",
				Help:      "Total number of RabbitMQ messages consumed by worker.",
			},
			[]string{"event_type", "status"},
		),

		messagesFailedTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: Namespace,
				Name:      "worker_messages_failed_total",
				Help:      "Total number of RabbitMQ messages failed by worker.",
			},
			[]string{"event_type"},
		),

		processingDurationSeconds: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: Namespace,
				Name:      "worker_processing_duration_seconds",
				Help:      "Worker message processing duration in seconds.",
				Buckets:   prometheus.DefBuckets,
			},
			[]string{"event_type", "status"},
		),

		retriesTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: Namespace,
				Name:      "worker_retries_total",
				Help:      "Total number of worker retry attempts.",
			},
			[]string{"event_type"},
		),

		activeJobs: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: Namespace,
				Name:      "worker_active_jobs",
				Help:      "Number of worker jobs currently being processed.",
			},
			[]string{"event_type"},
		),
	}

	registry.MustRegister(
		m.messagesConsumedTotal,
		m.messagesFailedTotal,
		m.processingDurationSeconds,
		m.retriesTotal,
		m.activeJobs,
	)

	return m
}

// StartWorkerJob отмечает начало обработки сообщения worker-ом.
func (m *WorkerMetrics) StartWorkerJob(eventType string) time.Time {
	if m == nil {
		return time.Now()
	}

	m.activeJobs.WithLabelValues(eventType).Inc()

	return time.Now()
}

// FinishWorkerJob отмечает завершение обработки сообщения worker-ом.
func (m *WorkerMetrics) FinishWorkerJob(eventType string, status string, startedAt time.Time) {
	if m == nil {
		return
	}

	m.activeJobs.WithLabelValues(eventType).Dec()
	m.messagesConsumedTotal.WithLabelValues(eventType, status).Inc()
	m.processingDurationSeconds.WithLabelValues(eventType, status).Observe(time.Since(startedAt).Seconds())

	if status == StatusError {
		m.messagesFailedTotal.WithLabelValues(eventType).Inc()
	}
}

// RecordWorkerRetry записывает retry-попытку worker-а.
func (m *WorkerMetrics) RecordWorkerRetry(eventType string) {
	if m == nil {
		return
	}

	m.retriesTotal.WithLabelValues(eventType).Inc()
}
