package worker

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"strings"

	"github.com/Dyuzhovsergey/gophprofile/internal/domain"
	"github.com/Dyuzhovsergey/gophprofile/internal/logger"
	"github.com/Dyuzhovsergey/gophprofile/internal/services"
)

// AvatarRepository описывает методы repository, которые нужны worker-у.
type AvatarRepository interface {
	GetByID(ctx context.Context, id string) (domain.Avatar, error)
	UpdateProcessingStatus(ctx context.Context, id string, status domain.ProcessingStatus) (domain.Avatar, error)
	UpdateProcessingResult(
		ctx context.Context,
		id string,
		width int,
		height int,
		thumbnails map[domain.ThumbnailSize]string,
		status domain.ProcessingStatus,
	) (domain.Avatar, error)
}

// AvatarStorage описывает методы файлового хранилища, которые нужны worker-у.
type AvatarStorage interface {
	Download(ctx context.Context, key string) ([]byte, string, error)
	Upload(ctx context.Context, key string, body io.Reader, contentType string) error
	Delete(ctx context.Context, key string) error
}

// ImageProcessor описывает сервис обработки изображений.
type ImageProcessor interface {
	Process(data []byte) (services.ImageProcessResult, error)
}

// AvatarProcessor обрабатывает события, связанные с аватарками.
type AvatarProcessor struct {
	log            *slog.Logger
	repo           AvatarRepository
	storage        AvatarStorage
	imageProcessor ImageProcessor
}

// NewAvatarProcessor создаёт обработчик событий аватарок.
func NewAvatarProcessor(
	log *slog.Logger,
	repo AvatarRepository,
	storage AvatarStorage,
	imageProcessor ImageProcessor,
) *AvatarProcessor {
	return &AvatarProcessor{
		log:            log,
		repo:           repo,
		storage:        storage,
		imageProcessor: imageProcessor,
	}
}

// HandleAvatarUploaded обрабатывает событие загрузки аватарки.
func (p *AvatarProcessor) HandleAvatarUploaded(ctx context.Context, event domain.AvatarUploadEvent) error {
	avatarID := strings.TrimSpace(event.AvatarID)
	if avatarID == "" {
		return domain.ErrAvatarNotFound
	}

	p.log.Info(
		"avatar uploaded event received",
		slog.String("avatar_id", event.AvatarID),
		slog.String("user_id", event.UserID),
		slog.String("s3_key", event.S3Key),
	)

	avatar, err := p.repo.GetByID(ctx, avatarID)
	if err != nil {
		p.log.Error(
			"failed to get avatar metadata",
			slog.String("avatar_id", avatarID),
			logger.Err(err),
		)

		return fmt.Errorf("get avatar metadata: %w", err)
	}

	if avatar.IsDeleted() {
		p.log.Info(
			"avatar is deleted, skipping uploaded event",
			slog.String("avatar_id", avatar.ID),
			slog.String("user_id", avatar.UserID),
		)

		return nil
	}

	if avatar.ProcessingStatus == domain.ProcessingStatusCompleted {
		p.log.Info(
			"avatar processing already completed, skipping event",
			slog.String("avatar_id", avatar.ID),
			slog.String("user_id", avatar.UserID),
			slog.String("processing_status", string(avatar.ProcessingStatus)),
		)

		return nil
	}

	if _, err := p.repo.UpdateProcessingStatus(ctx, avatar.ID, domain.ProcessingStatusProcessing); err != nil {
		return fmt.Errorf("set processing status: %w", err)
	}

	originalData, _, err := p.storage.Download(ctx, avatar.S3Key)
	if err != nil {
		p.log.Error(
			"failed to download original avatar",
			slog.String("avatar_id", avatar.ID),
			slog.String("s3_key", avatar.S3Key),
			logger.Err(err),
		)

		p.markProcessingFailed(ctx, avatar.ID)

		return fmt.Errorf("download original avatar: %w", err)
	}

	result, err := p.imageProcessor.Process(originalData)
	if err != nil {
		p.log.Error(
			"failed to process avatar image",
			slog.String("avatar_id", avatar.ID),
			slog.String("s3_key", avatar.S3Key),
			logger.Err(err),
		)

		p.markProcessingFailed(ctx, avatar.ID)

		return fmt.Errorf("process avatar image: %w", err)
	}

	thumbnailKeys := make(map[domain.ThumbnailSize]string, len(result.Thumbnails))

	for _, thumbnail := range result.Thumbnails {
		key := buildThumbnailS3Key(avatar.ID, thumbnail.Size, thumbnail.Extension)

		if err := p.storage.Upload(
			ctx,
			key,
			bytes.NewReader(thumbnail.Data),
			thumbnail.ContentType,
		); err != nil {
			p.markProcessingFailed(ctx, avatar.ID)

			return fmt.Errorf("upload thumbnail %s: %w", thumbnail.Size, err)
		}

		thumbnailKeys[thumbnail.Size] = key
	}

	if _, err := p.repo.UpdateProcessingResult(
		ctx,
		avatar.ID,
		result.Width,
		result.Height,
		thumbnailKeys,
		domain.ProcessingStatusCompleted,
	); err != nil {
		p.markProcessingFailed(ctx, avatar.ID)

		return fmt.Errorf("update processing result: %w", err)
	}

	p.log.Info(
		"avatar thumbnails generated",
		slog.String("avatar_id", avatar.ID),
		slog.Int("width", result.Width),
		slog.Int("height", result.Height),
		slog.Int("thumbnails_count", len(result.Thumbnails)),
	)

	return nil
}

// markProcessingFailed пытается сохранить статус failed при ошибке обработки.
func (p *AvatarProcessor) markProcessingFailed(ctx context.Context, avatarID string) {
	if _, err := p.repo.UpdateProcessingStatus(ctx, avatarID, domain.ProcessingStatusFailed); err != nil {
		p.log.Error(
			"failed to mark avatar processing as failed",
			slog.String("avatar_id", avatarID),
			logger.Err(err),
		)
	}
}

// buildThumbnailS3Key строит ключ thumbnail-файла в S3.
func buildThumbnailS3Key(
	avatarID string,
	size domain.ThumbnailSize,
	extension string,
) string {
	if strings.TrimSpace(extension) == "" {
		extension = ".jpg"
	}

	return fmt.Sprintf("thumbnails/%s/%s%s", avatarID, size, extension)
}

// HandleAvatarDeleted обрабатывает событие удаления аватарки.
func (p *AvatarProcessor) HandleAvatarDeleted(ctx context.Context, event domain.AvatarDeletedEvent) error {
	avatarID := strings.TrimSpace(event.AvatarID)
	if avatarID == "" {
		return domain.ErrAvatarNotFound
	}

	p.log.Info(
		"avatar deleted event received",
		slog.String("avatar_id", event.AvatarID),
		slog.String("user_id", event.UserID),
		slog.String("s3_key", event.S3Key),
		slog.Int("thumbnails_count", len(event.ThumbnailS3Keys)),
	)

	if strings.TrimSpace(event.S3Key) != "" {
		if err := p.storage.Delete(ctx, event.S3Key); err != nil {
			return fmt.Errorf("delete original avatar from s3: %w", err)
		}
	}

	for size, key := range event.ThumbnailS3Keys {
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}

		if err := p.storage.Delete(ctx, key); err != nil {
			return fmt.Errorf("delete avatar thumbnail %s from s3: %w", size, err)
		}
	}

	p.log.Info(
		"avatar files deleted from s3",
		slog.String("avatar_id", event.AvatarID),
		slog.String("user_id", event.UserID),
	)

	return nil
}
