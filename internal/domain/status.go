package domain

// UploadStatus описывает статус загрузки оригинального файла.
type UploadStatus string

const (
	// UploadStatusUploading означает, что файл загружается или запись только создаётся.
	UploadStatusUploading UploadStatus = "uploading"

	// UploadStatusUploaded означает, что оригинальный файл успешно сохранён.
	UploadStatusUploaded UploadStatus = "uploaded"

	// UploadStatusFailed означает, что загрузка оригинального файла завершилась ошибкой.
	UploadStatusFailed UploadStatus = "failed"
)

// IsValid проверяет, что статус загрузки поддерживается приложением.
func (s UploadStatus) IsValid() bool {
	switch s {
	case UploadStatusUploading, UploadStatusUploaded, UploadStatusFailed:
		return true
	default:
		return false
	}
}

// ProcessingStatus описывает статус фоновой обработки изображения.
type ProcessingStatus string

const (
	// ProcessingStatusPending означает, что обработка ожидает выполнения.
	ProcessingStatusPending ProcessingStatus = "pending"

	// ProcessingStatusProcessing означает, что worker сейчас обрабатывает изображение.
	ProcessingStatusProcessing ProcessingStatus = "processing"

	// ProcessingStatusCompleted означает, что обработка успешно завершена.
	ProcessingStatusCompleted ProcessingStatus = "completed"

	// ProcessingStatusFailed означает, что обработка завершилась ошибкой.
	ProcessingStatusFailed ProcessingStatus = "failed"
)

// IsValid проверяет, что статус обработки поддерживается приложением.
func (s ProcessingStatus) IsValid() bool {
	switch s {
	case ProcessingStatusPending,
		ProcessingStatusProcessing,
		ProcessingStatusCompleted,
		ProcessingStatusFailed:
		return true
	default:
		return false
	}
}

// ThumbnailSize описывает размер миниатюры изображения.
type ThumbnailSize string

const (
	// ThumbnailSizeOriginal означает оригинальное изображение.
	ThumbnailSizeOriginal ThumbnailSize = "original"

	// ThumbnailSize100 означает миниатюру 100x100.
	ThumbnailSize100 ThumbnailSize = "100x100"

	// ThumbnailSize300 означает миниатюру 300x300.
	ThumbnailSize300 ThumbnailSize = "300x300"
)

// IsValid проверяет, что размер миниатюры поддерживается приложением.
func (s ThumbnailSize) IsValid() bool {
	switch s {
	case ThumbnailSizeOriginal, ThumbnailSize100, ThumbnailSize300:
		return true
	default:
		return false
	}
}
