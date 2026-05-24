package metrics

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

const (
	// StatusSuccess используется для успешных бизнес-операций.
	StatusSuccess = "success"

	// StatusError используется для бизнес-операций, завершившихся ошибкой.
	StatusError = "error"

	// StatusSkipped используется для операций, которые были пропущены без ошибки.
	StatusSkipped = "skipped"
)

var (
	// AvatarUploadsTotal считает количество загрузок аватарок.
	AvatarUploadsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: Namespace,
			Name:      "avatars_uploads_total",
			Help:      "Total number of avatar upload attempts.",
		},
		[]string{"status"},
	)

	// AvatarUploadDurationSeconds измеряет длительность загрузки аватарки.
	AvatarUploadDurationSeconds = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: Namespace,
			Name:      "avatars_upload_duration_seconds",
			Help:      "Avatar upload duration in seconds.",
			Buckets:   prometheus.DefBuckets,
		},
		[]string{"status"},
	)

	// AvatarStorageBytes показывает примерный объём данных аватарок в хранилище.
	AvatarStorageBytes = promauto.NewGauge(
		prometheus.GaugeOpts{
			Namespace: Namespace,
			Name:      "avatars_storage_bytes",
			Help:      "Approximate number of bytes stored for avatars.",
		},
	)

	// AvatarDeletedTotal считает количество операций удаления аватарок.
	AvatarDeletedTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: Namespace,
			Name:      "avatars_deleted_total",
			Help:      "Total number of avatar delete attempts.",
		},
		[]string{"status"},
	)

	// AvatarProcessingTotal считает количество обработок аватарок worker-ом.
	AvatarProcessingTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: Namespace,
			Name:      "avatars_processing_total",
			Help:      "Total number of avatar processing attempts.",
		},
		[]string{"status"},
	)

	// AvatarProcessingDurationSeconds измеряет длительность обработки аватарок worker-ом.
	AvatarProcessingDurationSeconds = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: Namespace,
			Name:      "avatars_processing_duration_seconds",
			Help:      "Avatar processing duration in seconds.",
			Buckets:   prometheus.DefBuckets,
		},
		[]string{"status"},
	)

	// AvatarProcessingErrorsTotal считает ошибки обработки аватарок worker-ом.
	AvatarProcessingErrorsTotal = promauto.NewCounter(
		prometheus.CounterOpts{
			Namespace: Namespace,
			Name:      "avatars_processing_errors_total",
			Help:      "Total number of avatar processing errors.",
		},
	)
)

// RecordAvatarUpload записывает метрики загрузки аватарки.
func RecordAvatarUpload(status string, duration time.Duration) {
	AvatarUploadsTotal.WithLabelValues(status).Inc()
	AvatarUploadDurationSeconds.WithLabelValues(status).Observe(duration.Seconds())
}

// AddAvatarStorageBytes изменяет примерный объём хранилища аватарок.
func AddAvatarStorageBytes(delta int64) {
	AvatarStorageBytes.Add(float64(delta))
}

// RecordAvatarDelete записывает метрики удаления аватарки.
func RecordAvatarDelete(status string) {
	AvatarDeletedTotal.WithLabelValues(status).Inc()
}

// RecordAvatarProcessing записывает метрики обработки аватарки worker-ом.
func RecordAvatarProcessing(status string, duration time.Duration) {
	AvatarProcessingTotal.WithLabelValues(status).Inc()
	AvatarProcessingDurationSeconds.WithLabelValues(status).Observe(duration.Seconds())

	if status == StatusError {
		AvatarProcessingErrorsTotal.Inc()
	}
}
