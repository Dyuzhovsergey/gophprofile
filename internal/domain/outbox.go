package domain

import (
	"encoding/json"
	"time"
)

// OutboxEventType описывает тип события outbox.
type OutboxEventType string

const (
	// OutboxEventTypeAvatarUploaded означает событие загрузки аватарки.
	OutboxEventTypeAvatarUploaded OutboxEventType = "avatar.uploaded"

	// OutboxEventTypeAvatarDeleted означает событие удаления аватарки.
	OutboxEventTypeAvatarDeleted OutboxEventType = "avatar.deleted"
)

// IsValid проверяет, что тип события outbox поддерживается приложением.
func (t OutboxEventType) IsValid() bool {
	switch t {
	case OutboxEventTypeAvatarUploaded,
		OutboxEventTypeAvatarDeleted:
		return true
	default:
		return false
	}
}

// OutboxEventStatus описывает статус события outbox.
type OutboxEventStatus string

const (
	// OutboxEventStatusPending означает, что событие ожидает публикации.
	OutboxEventStatusPending OutboxEventStatus = "pending"

	// OutboxEventStatusPublished означает, что событие успешно опубликовано.
	OutboxEventStatusPublished OutboxEventStatus = "published"

	// OutboxEventStatusFailed означает, что событие временно не удалось опубликовать.
	OutboxEventStatusFailed OutboxEventStatus = "failed"
)

// IsValid проверяет, что статус outbox события поддерживается приложением.
func (s OutboxEventStatus) IsValid() bool {
	switch s {
	case OutboxEventStatusPending,
		OutboxEventStatusPublished,
		OutboxEventStatusFailed:
		return true
	default:
		return false
	}
}

// OutboxEvent описывает событие, которое нужно опубликовать во внешний брокер.
type OutboxEvent struct {
	ID          string
	EventType   OutboxEventType
	Payload     json.RawMessage
	Headers     map[string]string
	Status      OutboxEventStatus
	Attempts    int
	LastError   string
	AvailableAt time.Time
	PublishedAt *time.Time
	CreatedAt   time.Time
	UpdatedAt   time.Time
}
