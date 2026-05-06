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
)
