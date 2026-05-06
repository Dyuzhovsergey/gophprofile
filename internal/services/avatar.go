package services

import (
	"context"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/Dyuzhovsergey/gophprofile/internal/domain"
	"github.com/google/uuid"
)

// DefaultMaxUploadSizeBytes задаёт максимальный размер файла по умолчанию: 10 MB.
const DefaultMaxUploadSizeBytes int64 = 10 * 1024 * 1024

// AvatarRepository описывает методы хранилища метаданных аватарок.
type AvatarRepository interface {
	Create(ctx context.Context, avatar domain.Avatar) (domain.Avatar, error)
	GetByID(ctx context.Context, id string) (domain.Avatar, error)
	GetLatestByUserID(ctx context.Context, userID string) (domain.Avatar, error)
}

// AvatarStorage описывает методы файлового хранилища аватарок.
type AvatarStorage interface {
	Upload(ctx context.Context, key string, body io.Reader, contentType string) error
	Download(ctx context.Context, key string) ([]byte, string, error)
	Delete(ctx context.Context, key string) error
}

// AvatarService содержит бизнес-логику управления аватарками.
type AvatarService struct {
	repo               AvatarRepository
	storage            AvatarStorage
	maxUploadSizeBytes int64
}

// UploadAvatarInput содержит данные для загрузки аватарки.
type UploadAvatarInput struct {
	UserID    string
	FileName  string
	MIMEType  string
	SizeBytes int64
	Body      io.Reader
}

// DownloadAvatarResult содержит данные скачанной аватарки.
type DownloadAvatarResult struct {
	Avatar      domain.Avatar
	Data        []byte
	ContentType string
}

// NewAvatarService создаёт сервис управления аватарками.
func NewAvatarService(
	repo AvatarRepository,
	storage AvatarStorage,
	maxUploadSizeBytes int64,
) *AvatarService {
	if maxUploadSizeBytes <= 0 {
		maxUploadSizeBytes = DefaultMaxUploadSizeBytes
	}

	return &AvatarService{
		repo:               repo,
		storage:            storage,
		maxUploadSizeBytes: maxUploadSizeBytes,
	}
}

// UploadAvatar загружает файл аватарки в storage и сохраняет метаданные в repository.
func (s *AvatarService) UploadAvatar(ctx context.Context, input UploadAvatarInput) (domain.Avatar, error) {
	userID := strings.TrimSpace(input.UserID)
	if userID == "" {
		return domain.Avatar{}, domain.ErrMissingUserID
	}

	if input.Body == nil {
		return domain.Avatar{}, domain.ErrInvalidFile
	}

	if input.SizeBytes <= 0 {
		return domain.Avatar{}, domain.ErrInvalidFile
	}

	if input.SizeBytes > s.maxUploadSizeBytes {
		return domain.Avatar{}, domain.ErrFileTooLarge
	}

	mimeType := normalizeMIMEType(input.MIMEType)
	if !isAllowedAvatarMIMEType(mimeType) {
		return domain.Avatar{}, domain.ErrInvalidFile
	}

	avatarID := uuid.NewString()
	fileName := normalizeFileName(input.FileName)
	s3Key := buildOriginalS3Key(avatarID, fileName)

	avatar := domain.Avatar{
		ID:               avatarID,
		UserID:           userID,
		FileName:         fileName,
		MIMEType:         mimeType,
		SizeBytes:        input.SizeBytes,
		S3Key:            s3Key,
		ThumbnailS3Keys:  make(map[domain.ThumbnailSize]string),
		UploadStatus:     domain.UploadStatusUploaded,
		ProcessingStatus: domain.ProcessingStatusPending,
	}

	if err := s.storage.Upload(ctx, s3Key, input.Body, mimeType); err != nil {
		return domain.Avatar{}, fmt.Errorf("upload avatar file: %w", err)
	}

	createdAvatar, err := s.repo.Create(ctx, avatar)
	if err != nil {
		deleteErr := s.storage.Delete(ctx, s3Key)
		if deleteErr != nil {
			return domain.Avatar{}, fmt.Errorf(
				"create avatar metadata and cleanup uploaded file: %w",
				errors.Join(err, deleteErr),
			)
		}

		return domain.Avatar{}, fmt.Errorf("create avatar metadata: %w", err)
	}

	return createdAvatar, nil
}

// GetAvatarByID получает аватарку по id и скачивает файл из storage.
func (s *AvatarService) GetAvatarByID(ctx context.Context, avatarID string) (DownloadAvatarResult, error) {
	avatarID = strings.TrimSpace(avatarID)
	if avatarID == "" {
		return DownloadAvatarResult{}, domain.ErrAvatarNotFound
	}

	avatar, err := s.repo.GetByID(ctx, avatarID)
	if err != nil {
		return DownloadAvatarResult{}, err
	}

	if avatar.IsDeleted() {
		return DownloadAvatarResult{}, domain.ErrAvatarDeleted
	}

	data, contentType, err := s.storage.Download(ctx, avatar.S3Key)
	if err != nil {
		return DownloadAvatarResult{}, fmt.Errorf("download avatar file: %w", err)
	}

	if strings.TrimSpace(contentType) == "" {
		contentType = avatar.MIMEType
	}

	return DownloadAvatarResult{
		Avatar:      avatar,
		Data:        data,
		ContentType: contentType,
	}, nil
}

// GetCurrentAvatarByUserID получает последнюю активную аватарку пользователя.
func (s *AvatarService) GetCurrentAvatarByUserID(
	ctx context.Context,
	userID string,
) (DownloadAvatarResult, error) {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return DownloadAvatarResult{}, domain.ErrMissingUserID
	}

	avatar, err := s.repo.GetLatestByUserID(ctx, userID)
	if err != nil {
		return DownloadAvatarResult{}, err
	}

	if avatar.IsDeleted() {
		return DownloadAvatarResult{}, domain.ErrAvatarDeleted
	}

	data, contentType, err := s.storage.Download(ctx, avatar.S3Key)
	if err != nil {
		return DownloadAvatarResult{}, fmt.Errorf("download current avatar file: %w", err)
	}

	if strings.TrimSpace(contentType) == "" {
		contentType = avatar.MIMEType
	}

	return DownloadAvatarResult{
		Avatar:      avatar,
		Data:        data,
		ContentType: contentType,
	}, nil
}

// normalizeMIMEType приводит MIME-type к единому виду.
func normalizeMIMEType(mimeType string) string {
	return strings.ToLower(strings.TrimSpace(mimeType))
}

// isAllowedAvatarMIMEType проверяет, что формат изображения поддерживается.
func isAllowedAvatarMIMEType(mimeType string) bool {
	switch mimeType {
	case "image/jpeg", "image/png", "image/webp":
		return true
	default:
		return false
	}
}

// normalizeFileName возвращает безопасное имя файла без пути.
func normalizeFileName(fileName string) string {
	normalized := filepath.Base(strings.TrimSpace(fileName))
	if normalized == "" || normalized == "." || normalized == string(filepath.Separator) {
		return "avatar"
	}

	return normalized
}

// buildOriginalS3Key строит ключ оригинального файла в S3.
func buildOriginalS3Key(avatarID string, fileName string) string {
	return fmt.Sprintf("/original/%s/%s", avatarID, fileName)
}
