package metrics

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

const (
	// StatusSuccess используется для успешных бизнес-операций.
	StatusSuccess = "success"

	// StatusError используется для бизнес-операций, завершившихся ошибкой.
	StatusError = "error"

	// StatusSkipped используется для операций, которые были пропущены без ошибки.
	StatusSkipped = "skipped"
)

// AvatarMetrics содержит бизнес-метрики аватарок.
type AvatarMetrics struct {
	uploadsTotal              *prometheus.CounterVec
	uploadDurationSeconds     *prometheus.HistogramVec
	storageBytes              prometheus.Gauge
	deletedTotal              *prometheus.CounterVec
	processingTotal           *prometheus.CounterVec
	processingDurationSeconds *prometheus.HistogramVec
	processingErrorsTotal     prometheus.Counter
}

// NewAvatarMetrics создаёт и регистрирует бизнес-метрики аватарок.
func NewAvatarMetrics(registry prometheus.Registerer) *AvatarMetrics {
	m := &AvatarMetrics{
		uploadsTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: Namespace,
				Name:      "avatars_uploads_total",
				Help:      "Total number of avatar upload attempts.",
			},
			[]string{"status"},
		),

		uploadDurationSeconds: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: Namespace,
				Name:      "avatars_upload_duration_seconds",
				Help:      "Avatar upload duration in seconds.",
				Buckets:   prometheus.DefBuckets,
			},
			[]string{"status"},
		),

		storageBytes: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Namespace: Namespace,
				Name:      "avatars_storage_bytes",
				Help:      "Approximate number of bytes stored for avatars.",
			},
		),

		deletedTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: Namespace,
				Name:      "avatars_deleted_total",
				Help:      "Total number of avatar delete attempts.",
			},
			[]string{"status"},
		),

		processingTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: Namespace,
				Name:      "avatars_processing_total",
				Help:      "Total number of avatar processing attempts.",
			},
			[]string{"status"},
		),

		processingDurationSeconds: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: Namespace,
				Name:      "avatars_processing_duration_seconds",
				Help:      "Avatar processing duration in seconds.",
				Buckets:   prometheus.DefBuckets,
			},
			[]string{"status"},
		),

		processingErrorsTotal: prometheus.NewCounter(
			prometheus.CounterOpts{
				Namespace: Namespace,
				Name:      "avatars_processing_errors_total",
				Help:      "Total number of avatar processing errors.",
			},
		),
	}

	registry.MustRegister(
		m.uploadsTotal,
		m.uploadDurationSeconds,
		m.storageBytes,
		m.deletedTotal,
		m.processingTotal,
		m.processingDurationSeconds,
		m.processingErrorsTotal,
	)

	return m
}

// RecordAvatarUpload записывает метрики загрузки аватарки.
func (m *AvatarMetrics) RecordAvatarUpload(status string, duration time.Duration) {
	if m == nil {
		return
	}

	m.uploadsTotal.WithLabelValues(status).Inc()
	m.uploadDurationSeconds.WithLabelValues(status).Observe(duration.Seconds())
}

// AddAvatarStorageBytes изменяет примерный объём хранилища аватарок.
func (m *AvatarMetrics) AddAvatarStorageBytes(delta int64) {
	if m == nil {
		return
	}

	m.storageBytes.Add(float64(delta))
}

// RecordAvatarDelete записывает метрики удаления аватарки.
func (m *AvatarMetrics) RecordAvatarDelete(status string) {
	if m == nil {
		return
	}

	m.deletedTotal.WithLabelValues(status).Inc()
}

// RecordAvatarProcessing записывает метрики обработки аватарки worker-ом.
func (m *AvatarMetrics) RecordAvatarProcessing(status string, duration time.Duration) {
	if m == nil {
		return
	}

	m.processingTotal.WithLabelValues(status).Inc()
	m.processingDurationSeconds.WithLabelValues(status).Observe(duration.Seconds())

	if status == StatusError {
		m.processingErrorsTotal.Inc()
	}
}
