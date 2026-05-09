package domain

// AvatarUploadEvent описывает событие загрузки аватарки.
type AvatarUploadEvent struct {
	AvatarID string `json:"avatar_id"`
	UserID   string `json:"user_id"`
	S3Key    string `json:"s3_key"`
}

// AvatarDeletedEvent описывает событие удаления аватарки.
type AvatarDeletedEvent struct {
	AvatarID        string                   `json:"avatar_id"`
	UserID          string                   `json:"user_id"`
	S3Key           string                   `json:"s3_key"`
	ThumbnailS3Keys map[ThumbnailSize]string `json:"thumbnail_s3_keys"`
}
