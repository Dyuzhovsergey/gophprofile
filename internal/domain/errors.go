package domain

import "errors"

var (
	// ErrAvatarNotFound означает, что аватарка не найдена.
	ErrAvatarNotFound = errors.New("avatar not found")

	// ErrAvatarDeleted означает, что аватарка была удалена.
	ErrAvatarDeleted = errors.New("avatar deleted")

	// ErrForbidden означает, что действие запрещено для текущего пользователя.
	ErrForbidden = errors.New("forbidden")

	// ErrMissingUserID означает, что не передан обязательный user_id.
	ErrMissingUserID = errors.New("missing user id")

	// ErrInvalidFile означает, что переданный файл не подходит для аватарки.
	ErrInvalidFile = errors.New("invalid file")

	// ErrFileTooLarge означает, что файл превышает допустимый размер.
	ErrFileTooLarge = errors.New("file too large")

	// ErrInvalidStatus означает, что передан неизвестный статус.
	ErrInvalidStatus = errors.New("invalid status")

	// ErrInvalidThumbnailSize означает, что передан неподдерживаемый размер миниатюры.
	ErrInvalidThumbnailSize = errors.New("invalid thumbnail size")

	// ErrThumbnailNotFound означает, что миниатюра не найдена.
	ErrThumbnailNotFound = errors.New("thumbnail not found")

	// ErrInvalidOutboxEventType означает, что передан неизвестный тип outbox-события.
	ErrInvalidOutboxEventType = errors.New("invalid outbox event type")

	// ErrInvalidOutboxEventStatus означает, что передан неизвестный статус outbox-события.
	ErrInvalidOutboxEventStatus = errors.New("invalid outbox event status")

	// ErrInvalidOutboxPayload означает, что payload outbox-события пустой или некорректный.
	ErrInvalidOutboxPayload = errors.New("invalid outbox payload")
)
