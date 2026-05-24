package metrics

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// WorkerMessagesConsumedTotal считает количество сообщений, обработанных worker-ом.
	WorkerMessagesConsumedTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: Namespace,
			Name:      "worker_messages_consumed_total",
			Help:      "Total number of RabbitMQ messages consumed by worker.",
		},
		[]string{"event_type", "status"},
	)

	// WorkerMessagesFailedTotal считает количество сообщений, которые worker не смог обработать.
	WorkerMessagesFailedTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: Namespace,
			Name:      "worker_messages_failed_total",
			Help:      "Total number of RabbitMQ messages failed by worker.",
		},
		[]string{"event_type"},
	)

	// WorkerProcessingDurationSeconds измеряет длительность обработки сообщений worker-ом.
	WorkerProcessingDurationSeconds = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: Namespace,
			Name:      "worker_processing_duration_seconds",
			Help:      "Worker message processing duration in seconds.",
			Buckets:   prometheus.DefBuckets,
		},
		[]string{"event_type", "status"},
	)

	// WorkerRetriesTotal считает количество retry-попыток worker-а.
	//
	// Сейчас retry-логика явно не реализована, но метрика подготовлена
	// для следующих инкрементов над надёжностью обработки.
	WorkerRetriesTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: Namespace,
			Name:      "worker_retries_total",
			Help:      "Total number of worker retry attempts.",
		},
		[]string{"event_type"},
	)

	// WorkerActiveJobs показывает количество сообщений, которые worker обрабатывает прямо сейчас.
	WorkerActiveJobs = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: Namespace,
			Name:      "worker_active_jobs",
			Help:      "Number of worker jobs currently being processed.",
		},
		[]string{"event_type"},
	)
)

// StartWorkerJob отмечает начало обработки сообщения worker-ом.
func StartWorkerJob(eventType string) time.Time {
	WorkerActiveJobs.WithLabelValues(eventType).Inc()

	return time.Now()
}

// FinishWorkerJob отмечает завершение обработки сообщения worker-ом.
func FinishWorkerJob(eventType string, status string, startedAt time.Time) {
	WorkerActiveJobs.WithLabelValues(eventType).Dec()
	WorkerMessagesConsumedTotal.WithLabelValues(eventType, status).Inc()
	WorkerProcessingDurationSeconds.WithLabelValues(eventType, status).Observe(time.Since(startedAt).Seconds())

	if status == StatusError {
		WorkerMessagesFailedTotal.WithLabelValues(eventType).Inc()
	}
}

// RecordWorkerRetry записывает retry-попытку worker-а.
func RecordWorkerRetry(eventType string) {
	WorkerRetriesTotal.WithLabelValues(eventType).Inc()
}
