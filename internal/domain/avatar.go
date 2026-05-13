package domain

import (
	"strings"
	"time"
)

// Avatar описывает аватарку пользователя и её метаданные.
type Avatar struct {
	ID               string
	UserID           string
	FileName         string
	MIMEType         string
	SizeBytes        int64
	Width            int
	Height           int
	S3Key            string
	ThumbnailS3Keys  map[ThumbnailSize]string
	UploadStatus     UploadStatus
	ProcessingStatus ProcessingStatus
	CreatedAt        time.Time
	UpdatedAt        time.Time
	DeletedAt        *time.Time
}

// IsDeleted возвращает true, если аватарка была мягко удалена.
func (a Avatar) IsDeleted() bool {
	return a.DeletedAt != nil
}

// IsOwner возвращает true, если аватарка принадлежит указанному пользователю.
func (a Avatar) IsOwner(userID string) bool {
	return strings.TrimSpace(userID) != "" && a.UserID == userID
}

// HasThumbnail возвращает true, если для указанного размера есть ключ файла в S3.
func (a Avatar) HasThumbnail(size ThumbnailSize) bool {
	if a.ThumbnailS3Keys == nil {
		return false
	}

	key := strings.TrimSpace(a.ThumbnailS3Keys[size])

	return key != ""
}
