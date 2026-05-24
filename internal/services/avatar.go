package services

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/Dyuzhovsergey/gophprofile/internal/domain"
	"github.com/Dyuzhovsergey/gophprofile/internal/logger"
	observabilitylogging "github.com/Dyuzhovsergey/gophprofile/internal/observability/logging"
	observabilitymetrics "github.com/Dyuzhovsergey/gophprofile/internal/observability/metrics"
	observabilitytracing "github.com/Dyuzhovsergey/gophprofile/internal/observability/tracing"
	"github.com/google/uuid"
	"go.opentelemetry.io/otel/attribute"
)

// DefaultMaxUploadSizeBytes задаёт максимальный размер файла по умолчанию: 10 MB.
const DefaultMaxUploadSizeBytes int64 = 10 * 1024 * 1024

// detectContentTypeBufferSize задаёт количество байт для определения реального типа файла.
const detectContentTypeBufferSize = 512

// AvatarRepository описывает методы хранилища метаданных аватарок.
type AvatarRepository interface {
	CreateWithUploadEvent(ctx context.Context, avatar domain.Avatar) (domain.Avatar, error)
	GetByID(ctx context.Context, id string) (domain.Avatar, error)
	GetLatestByUserID(ctx context.Context, userID string) (domain.Avatar, error)
	ListByUserID(ctx context.Context, userID string) ([]domain.Avatar, error)
	SoftDeleteWithDeleteEvent(ctx context.Context, id string) (domain.Avatar, error)
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
	log                *slog.Logger
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
// NewAvatarService создаёт сервис управления аватарками.
func NewAvatarService(
	repo AvatarRepository,
	storage AvatarStorage,
	maxUploadSizeBytes int64,
	loggers ...*slog.Logger,
) *AvatarService {
	if maxUploadSizeBytes <= 0 {
		maxUploadSizeBytes = DefaultMaxUploadSizeBytes
	}

	log := logger.NewNop()
	if len(loggers) > 0 && loggers[0] != nil {
		log = loggers[0]
	}

	return &AvatarService{
		repo:               repo,
		storage:            storage,
		maxUploadSizeBytes: maxUploadSizeBytes,
		log:                log,
	}
}

// UploadAvatar загружает файл аватарки в storage и сохраняет метаданные в repository.
func (s *AvatarService) UploadAvatar(ctx context.Context, input UploadAvatarInput) (avatar domain.Avatar, err error) {
	ctx, span := observabilitytracing.StartSpan(
		ctx,
		"avatar_service.upload_avatar",
		attribute.String("user_id", strings.TrimSpace(input.UserID)),
		attribute.String("file_name", input.FileName),
		attribute.String("mime_type", input.MIMEType),
		attribute.Int64("file_size", input.SizeBytes),
	)
	defer func() {
		observabilitytracing.RecordError(span, err)
		span.End()
	}()

	startedAt := time.Now()

	defer func() {
		status := observabilitymetrics.StatusSuccess
		if err != nil {
			status = observabilitymetrics.StatusError
		}

		observabilitymetrics.RecordAvatarUpload(status, time.Since(startedAt))

		if err == nil {
			observabilitymetrics.AddAvatarStorageBytes(input.SizeBytes)
		}
	}()

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

	detectedMIMEType, body, err := detectAvatarMIMEType(input.Body)
	if err != nil {
		return domain.Avatar{}, err
	}

	if detectedMIMEType != mimeType {
		return domain.Avatar{}, domain.ErrInvalidFile
	}

	avatarID := uuid.NewString()

	span.SetAttributes(attribute.String("avatar_id", avatarID))

	fileName := normalizeFileName(input.FileName)
	s3Key := buildOriginalS3Key(avatarID, fileName)

	s.log.LogAttrs(
		ctx,
		slog.LevelInfo,
		"uploading avatar",
		observabilitylogging.AppendTraceAttrs(
			ctx,
			slog.String("user_id", userID),
			slog.String("file_name", fileName),
			slog.String("mime_type", mimeType),
			slog.Int64("file_size", input.SizeBytes),
			slog.String("s3_key", s3Key),
		)...,
	)

	avatar = domain.Avatar{
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

	if err := s.storage.Upload(ctx, s3Key, body, mimeType); err != nil {
		return domain.Avatar{}, fmt.Errorf("upload avatar file: %w", err)
	}

	createdAvatar, err := s.repo.CreateWithUploadEvent(ctx, avatar)
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

	s.log.LogAttrs(
		ctx,
		slog.LevelInfo,
		"avatar uploaded",
		observabilitylogging.AppendTraceAttrs(
			ctx,
			slog.String("avatar_id", createdAvatar.ID),
			slog.String("user_id", createdAvatar.UserID),
			slog.String("processing_status", string(createdAvatar.ProcessingStatus)),
		)...,
	)

	return createdAvatar, nil
}

// GetAvatarByID получает аватарку по id и скачивает файл из storage.
func (s *AvatarService) GetAvatarByID(ctx context.Context, avatarID string) (result DownloadAvatarResult, err error) {
	ctx, span := observabilitytracing.StartSpan(
		ctx,
		"avatar_service.get_avatar_by_id",
		attribute.String("avatar_id", strings.TrimSpace(avatarID)),
	)
	defer func() {
		observabilitytracing.RecordError(span, err)
		span.End()
	}()

	avatarID = strings.TrimSpace(avatarID)
	if avatarID == "" {
		return DownloadAvatarResult{}, domain.ErrAvatarNotFound
	}

	avatar, err := s.repo.GetByID(ctx, avatarID)
	if err != nil {
		return DownloadAvatarResult{}, err
	}

	span.SetAttributes(
		attribute.String("user_id", avatar.UserID),
		attribute.String("s3_key", avatar.S3Key),
	)

	if avatar.IsDeleted() {
		return DownloadAvatarResult{}, domain.ErrAvatarDeleted
	}

	s.log.LogAttrs(
		ctx,
		slog.LevelInfo,
		"getting avatar by id",
		observabilitylogging.AppendTraceAttrs(
			ctx,
			slog.String("avatar_id", avatar.ID),
			slog.String("user_id", avatar.UserID),
			slog.String("s3_key", avatar.S3Key),
		)...,
	)

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

// GetAvatarThumbnailByID получает миниатюру аватарки по id и размеру.
func (s *AvatarService) GetAvatarThumbnailByID(
	ctx context.Context,
	avatarID string,
	size domain.ThumbnailSize,
) (result DownloadAvatarResult, err error) {
	ctx, span := observabilitytracing.StartSpan(
		ctx,
		"avatar_service.get_avatar_thumbnail_by_id",
		attribute.String("avatar_id", strings.TrimSpace(avatarID)),
		attribute.String("thumbnail_size", string(size)),
	)
	defer func() {
		observabilitytracing.RecordError(span, err)
		span.End()
	}()

	avatarID = strings.TrimSpace(avatarID)
	if avatarID == "" {
		return DownloadAvatarResult{}, domain.ErrAvatarNotFound
	}

	if !size.IsValid() || size == domain.ThumbnailSizeOriginal {
		return DownloadAvatarResult{}, domain.ErrInvalidThumbnailSize
	}

	avatar, err := s.repo.GetByID(ctx, avatarID)
	if err != nil {
		return DownloadAvatarResult{}, err
	}

	span.SetAttributes(
		attribute.String("user_id", avatar.UserID),
		attribute.String("s3_key", avatar.S3Key),
	)

	if avatar.IsDeleted() {
		return DownloadAvatarResult{}, domain.ErrAvatarDeleted
	}

	thumbnailKey := strings.TrimSpace(avatar.ThumbnailS3Keys[size])
	if thumbnailKey == "" {
		return DownloadAvatarResult{}, domain.ErrThumbnailNotFound
	}

	span.SetAttributes(attribute.String("thumbnail_s3_key", thumbnailKey))

	data, contentType, err := s.storage.Download(ctx, thumbnailKey)
	if err != nil {
		return DownloadAvatarResult{}, fmt.Errorf("download avatar thumbnail: %w", err)
	}

	if strings.TrimSpace(contentType) == "" {
		contentType = "image/jpeg"
	}

	return DownloadAvatarResult{
		Avatar:      avatar,
		Data:        data,
		ContentType: contentType,
	}, nil
}

// GetAvatarMetadata получает метаданные аватарки по id без скачивания файла из storage.
func (s *AvatarService) GetAvatarMetadata(ctx context.Context, avatarID string) (avatar domain.Avatar, err error) {
	ctx, span := observabilitytracing.StartSpan(
		ctx,
		"avatar_service.get_avatar_metadata",
		attribute.String("avatar_id", strings.TrimSpace(avatarID)),
	)
	defer func() {
		observabilitytracing.RecordError(span, err)
		span.End()
	}()

	avatarID = strings.TrimSpace(avatarID)
	if avatarID == "" {
		return domain.Avatar{}, domain.ErrAvatarNotFound
	}

	avatar, err = s.repo.GetByID(ctx, avatarID)
	if err != nil {
		return domain.Avatar{}, err
	}

	span.SetAttributes(
		attribute.String("user_id", avatar.UserID),
		attribute.String("processing_status", string(avatar.ProcessingStatus)),
	)

	if avatar.IsDeleted() {
		return domain.Avatar{}, domain.ErrAvatarDeleted
	}

	return avatar, nil
}

// GetCurrentAvatarByUserID получает последнюю активную аватарку пользователя.
func (s *AvatarService) GetCurrentAvatarByUserID(
	ctx context.Context,
	userID string,
) (result DownloadAvatarResult, err error) {
	ctx, span := observabilitytracing.StartSpan(
		ctx,
		"avatar_service.get_current_avatar_by_user_id",
		attribute.String("user_id", strings.TrimSpace(userID)),
	)
	defer func() {
		observabilitytracing.RecordError(span, err)
		span.End()
	}()

	userID = strings.TrimSpace(userID)
	if userID == "" {
		return DownloadAvatarResult{}, domain.ErrMissingUserID
	}

	avatar, err := s.repo.GetLatestByUserID(ctx, userID)
	if err != nil {
		return DownloadAvatarResult{}, err
	}

	span.SetAttributes(
		attribute.String("avatar_id", avatar.ID),
		attribute.String("s3_key", avatar.S3Key),
	)

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

// ListAvatarsByUserID возвращает список активных аватарок пользователя.
func (s *AvatarService) ListAvatarsByUserID(ctx context.Context, userID string) (avatars []domain.Avatar, err error) {
	ctx, span := observabilitytracing.StartSpan(
		ctx,
		"avatar_service.list_avatars_by_user_id",
		attribute.String("user_id", strings.TrimSpace(userID)),
	)
	defer func() {
		observabilitytracing.RecordError(span, err)
		span.End()
	}()

	userID = strings.TrimSpace(userID)
	if userID == "" {
		return nil, domain.ErrMissingUserID
	}

	avatars, err = s.repo.ListByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}

	span.SetAttributes(attribute.Int("avatars_count", len(avatars)))

	return avatars, nil
}

// DeleteAvatarByID мягко удаляет аватарку по id, если она принадлежит пользователю.
func (s *AvatarService) DeleteAvatarByID(
	ctx context.Context,
	avatarID string,
	userID string,
) (deletedAvatar domain.Avatar, err error) {
	ctx, span := observabilitytracing.StartSpan(
		ctx,
		"avatar_service.delete_avatar_by_id",
		attribute.String("avatar_id", strings.TrimSpace(avatarID)),
		attribute.String("user_id", strings.TrimSpace(userID)),
	)
	defer func() {
		observabilitytracing.RecordError(span, err)
		span.End()
	}()

	defer func() {
		status := observabilitymetrics.StatusSuccess
		if err != nil {
			status = observabilitymetrics.StatusError
		}

		observabilitymetrics.RecordAvatarDelete(status)

		if err == nil {
			observabilitymetrics.AddAvatarStorageBytes(-deletedAvatar.SizeBytes)
		}
	}()

	avatarID = strings.TrimSpace(avatarID)
	if avatarID == "" {
		return domain.Avatar{}, domain.ErrAvatarNotFound
	}

	userID = strings.TrimSpace(userID)
	if userID == "" {
		return domain.Avatar{}, domain.ErrMissingUserID
	}

	avatar, err := s.repo.GetByID(ctx, avatarID)
	if err != nil {
		return domain.Avatar{}, err
	}

	span.SetAttributes(
		attribute.String("avatar_owner_id", avatar.UserID),
		attribute.String("s3_key", avatar.S3Key),
	)

	if avatar.IsDeleted() {
		return domain.Avatar{}, domain.ErrAvatarDeleted
	}

	if avatar.UserID != userID {
		return domain.Avatar{}, domain.ErrForbidden
	}

	s.log.LogAttrs(
		ctx,
		slog.LevelInfo,
		"deleting avatar by id",
		observabilitylogging.AppendTraceAttrs(
			ctx,
			slog.String("avatar_id", avatar.ID),
			slog.String("user_id", userID),
		)...,
	)

	deletedAvatar, err = s.repo.SoftDeleteWithDeleteEvent(ctx, avatarID)
	if err != nil {
		return domain.Avatar{}, err
	}

	span.SetAttributes(attribute.Bool("deleted", true))

	s.log.LogAttrs(
		ctx,
		slog.LevelInfo,
		"avatar deleted by id",
		observabilitylogging.AppendTraceAttrs(
			ctx,
			slog.String("avatar_id", deletedAvatar.ID),
			slog.String("user_id", deletedAvatar.UserID),
		)...,
	)

	return deletedAvatar, nil
}

// DeleteCurrentAvatarByUserID мягко удаляет последнюю активную аватарку пользователя.
func (s *AvatarService) DeleteCurrentAvatarByUserID(
	ctx context.Context,
	userID string,
	actorUserID string,
) (deletedAvatar domain.Avatar, err error) {
	ctx, span := observabilitytracing.StartSpan(
		ctx,
		"avatar_service.delete_current_avatar_by_user_id",
		attribute.String("user_id", strings.TrimSpace(userID)),
		attribute.String("actor_user_id", strings.TrimSpace(actorUserID)),
	)
	defer func() {
		observabilitytracing.RecordError(span, err)
		span.End()
	}()

	defer func() {
		status := observabilitymetrics.StatusSuccess
		if err != nil {
			status = observabilitymetrics.StatusError
		}

		observabilitymetrics.RecordAvatarDelete(status)

		if err == nil {
			observabilitymetrics.AddAvatarStorageBytes(-deletedAvatar.SizeBytes)
		}
	}()

	userID = strings.TrimSpace(userID)
	if userID == "" {
		return domain.Avatar{}, domain.ErrMissingUserID
	}

	actorUserID = strings.TrimSpace(actorUserID)
	if actorUserID == "" {
		return domain.Avatar{}, domain.ErrMissingUserID
	}

	if userID != actorUserID {
		return domain.Avatar{}, domain.ErrForbidden
	}

	avatar, err := s.repo.GetLatestByUserID(ctx, userID)
	if err != nil {
		return domain.Avatar{}, err
	}

	span.SetAttributes(
		attribute.String("avatar_id", avatar.ID),
		attribute.String("s3_key", avatar.S3Key),
	)

	if avatar.IsDeleted() {
		return domain.Avatar{}, domain.ErrAvatarDeleted
	}

	if !avatar.IsOwner(actorUserID) {
		return domain.Avatar{}, domain.ErrForbidden
	}

	deletedAvatar, err = s.repo.SoftDeleteWithDeleteEvent(ctx, avatar.ID)
	if err != nil {
		return domain.Avatar{}, err
	}

	span.SetAttributes(attribute.Bool("deleted", true))

	return deletedAvatar, nil
}

// detectAvatarMIMEType определяет реальный MIME-type файла по первым байтам.
func detectAvatarMIMEType(body io.Reader) (string, io.Reader, error) {
	buffer := make([]byte, detectContentTypeBufferSize)

	n, err := io.ReadFull(body, buffer)
	if err != nil && !errors.Is(err, io.EOF) && !errors.Is(err, io.ErrUnexpectedEOF) {
		return "", nil, fmt.Errorf("read avatar magic bytes: %w", err)
	}

	if n == 0 {
		return "", nil, domain.ErrInvalidFile
	}

	buffer = buffer[:n]

	mimeType := normalizeMIMEType(http.DetectContentType(buffer))
	if !isAllowedAvatarMIMEType(mimeType) {
		return "", nil, domain.ErrInvalidFile
	}

	return mimeType, io.MultiReader(bytes.NewReader(buffer), body), nil
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
	return fmt.Sprintf("originals/%s/%s", avatarID, fileName)
}
