package worker

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"time"

	"github.com/Dyuzhovsergey/gophprofile/internal/domain"
	observabilitylogging "github.com/Dyuzhovsergey/gophprofile/internal/observability/logging"
	observabilitymetrics "github.com/Dyuzhovsergey/gophprofile/internal/observability/metrics"
	observabilitytracing "github.com/Dyuzhovsergey/gophprofile/internal/observability/tracing"
	"github.com/Dyuzhovsergey/gophprofile/internal/services"
	"go.opentelemetry.io/otel/attribute"
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
	avatarMetrics  *observabilitymetrics.AvatarMetrics
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

// WithAvatarMetrics подключает бизнес-метрики аватарок к processor-у.
func (p *AvatarProcessor) WithAvatarMetrics(metrics *observabilitymetrics.AvatarMetrics) *AvatarProcessor {
	p.avatarMetrics = metrics

	return p
}

// HandleAvatarUploaded обрабатывает событие загрузки аватарки.
func (p *AvatarProcessor) HandleAvatarUploaded(ctx context.Context, event domain.AvatarUploadEvent) (err error) {
	ctx, span := observabilitytracing.StartSpan(
		ctx,
		"worker.process_avatar_uploaded",
		attribute.String("avatar_id", strings.TrimSpace(event.AvatarID)),
		attribute.String("user_id", strings.TrimSpace(event.UserID)),
		attribute.String("s3_key", strings.TrimSpace(event.S3Key)),
	)
	defer func() {
		observabilitytracing.RecordError(span, err)
		span.End()
	}()

	startedAt := time.Now()
	processingStatus := observabilitymetrics.StatusSuccess

	defer func() {
		if err != nil {
			processingStatus = observabilitymetrics.StatusError
		}

		p.avatarMetrics.RecordAvatarProcessing(processingStatus, time.Since(startedAt))
	}()

	avatarID := strings.TrimSpace(event.AvatarID)
	if avatarID == "" {
		return domain.ErrAvatarNotFound
	}

	p.log.LogAttrs(
		ctx,
		slog.LevelInfo,
		"avatar uploaded event received",
		observabilitylogging.AppendTraceAttrs(
			ctx,
			slog.String("avatar_id", event.AvatarID),
			slog.String("user_id", event.UserID),
			slog.String("s3_key", event.S3Key),
		)...,
	)

	avatar, err := p.repo.GetByID(ctx, avatarID)
	if err != nil {
		p.log.LogAttrs(
			ctx,
			slog.LevelError,
			"failed to get avatar metadata",
			observabilitylogging.ErrorAttrs(
				ctx,
				observabilitylogging.ComponentWorker,
				"worker.get_avatar_metadata",
				err,
				slog.String("avatar_id", avatarID),
			)...,
		)

		return fmt.Errorf("get avatar metadata: %w", err)
	}

	if avatar.IsDeleted() {
		p.log.LogAttrs(
			ctx,
			slog.LevelInfo,
			"avatar is deleted, skipping uploaded event",
			observabilitylogging.AppendTraceAttrs(
				ctx,
				slog.String("avatar_id", avatar.ID),
				slog.String("user_id", avatar.UserID),
			)...,
		)

		processingStatus = observabilitymetrics.StatusSkipped

		return nil
	}

	if avatar.ProcessingStatus == domain.ProcessingStatusCompleted {
		p.log.LogAttrs(
			ctx,
			slog.LevelInfo,
			"avatar processing already completed, skipping event",
			observabilitylogging.AppendTraceAttrs(
				ctx,
				slog.String("avatar_id", avatar.ID),
				slog.String("user_id", avatar.UserID),
				slog.String("processing_status", string(avatar.ProcessingStatus)),
			)...,
		)

		processingStatus = observabilitymetrics.StatusSkipped

		return nil
	}

	if _, err = p.repo.UpdateProcessingStatus(ctx, avatar.ID, domain.ProcessingStatusProcessing); err != nil {
		return fmt.Errorf("set processing status: %w", err)
	}

	originalData, _, err := p.storage.Download(ctx, avatar.S3Key)
	if err != nil {
		p.log.LogAttrs(
			ctx,
			slog.LevelError,
			"failed to download original avatar",
			observabilitylogging.ErrorAttrs(
				ctx,
				observabilitylogging.ComponentS3,
				"s3.download_original_avatar",
				err,
				slog.String("avatar_id", avatar.ID),
				slog.String("user_id", avatar.UserID),
				slog.String("s3_key", avatar.S3Key),
			)...,
		)

		p.markProcessingFailed(ctx, avatar.ID)

		return fmt.Errorf("download original avatar: %w", err)
	}

	_, generateSpan := observabilitytracing.StartSpan(
		ctx,
		"worker.generate_thumbnails",
		attribute.String("avatar_id", avatar.ID),
		attribute.String("user_id", avatar.UserID),
		attribute.Int64("original_file_size", int64(len(originalData))),
	)

	result, err := p.imageProcessor.Process(originalData)
	if err != nil {
		observabilitytracing.RecordError(generateSpan, err)
		generateSpan.End()

		p.log.LogAttrs(
			ctx,
			slog.LevelError,
			"failed to process avatar image",
			observabilitylogging.ErrorAttrs(
				ctx,
				observabilitylogging.ComponentWorker,
				"worker.generate_thumbnails",
				err,
				slog.String("avatar_id", avatar.ID),
				slog.String("user_id", avatar.UserID),
				slog.String("s3_key", avatar.S3Key),
			)...,
		)

		p.markProcessingFailed(ctx, avatar.ID)

		return fmt.Errorf("process avatar image: %w", err)
	}

	generateSpan.SetAttributes(
		attribute.Int("width", result.Width),
		attribute.Int("height", result.Height),
		attribute.Int("thumbnails_count", len(result.Thumbnails)),
	)
	generateSpan.End()

	var thumbnailsSizeBytes int64
	for _, thumbnail := range result.Thumbnails {
		thumbnailsSizeBytes += int64(len(thumbnail.Data))
	}

	span.SetAttributes(attribute.Int64("thumbnails_size_bytes", thumbnailsSizeBytes))

	thumbnailKeys := make(map[domain.ThumbnailSize]string, len(result.Thumbnails))

	for _, thumbnail := range result.Thumbnails {
		key := buildThumbnailS3Key(avatar.ID, thumbnail.Size, thumbnail.Extension)

		if err = p.storage.Upload(
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

	if _, err = p.repo.UpdateProcessingResult(
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

	p.log.LogAttrs(
		ctx,
		slog.LevelInfo,
		"avatar thumbnails generated",
		observabilitylogging.AppendTraceAttrs(
			ctx,
			slog.String("avatar_id", avatar.ID),
			slog.Int("width", result.Width),
			slog.Int("height", result.Height),
			slog.Int("thumbnails_count", len(result.Thumbnails)),
		)...,
	)

	span.SetAttributes(
		attribute.String("processing_status", string(domain.ProcessingStatusCompleted)),
		attribute.Int("width", result.Width),
		attribute.Int("height", result.Height),
		attribute.Int("thumbnails_count", len(result.Thumbnails)),
	)

	p.avatarMetrics.AddAvatarStorageBytes(thumbnailsSizeBytes)

	return nil
}

// markProcessingFailed пытается сохранить статус failed при ошибке обработки.
func (p *AvatarProcessor) markProcessingFailed(ctx context.Context, avatarID string) {
	if _, err := p.repo.UpdateProcessingStatus(ctx, avatarID, domain.ProcessingStatusFailed); err != nil {
		p.log.LogAttrs(
			ctx,
			slog.LevelError,
			"failed to mark avatar processing as failed",
			observabilitylogging.ErrorAttrs(
				ctx,
				observabilitylogging.ComponentWorker,
				"worker.mark_processing_failed",
				err,
				slog.String("avatar_id", avatarID),
			)...,
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
func (p *AvatarProcessor) HandleAvatarDeleted(ctx context.Context, event domain.AvatarDeletedEvent) (err error) {
	ctx, span := observabilitytracing.StartSpan(
		ctx,
		"worker.process_avatar_deleted",
		attribute.String("avatar_id", strings.TrimSpace(event.AvatarID)),
		attribute.String("user_id", strings.TrimSpace(event.UserID)),
		attribute.String("s3_key", strings.TrimSpace(event.S3Key)),
		attribute.Int("thumbnails_count", len(event.ThumbnailS3Keys)),
	)
	defer func() {
		observabilitytracing.RecordError(span, err)
		span.End()
	}()

	avatarID := strings.TrimSpace(event.AvatarID)
	if avatarID == "" {
		return domain.ErrAvatarNotFound
	}

	p.log.LogAttrs(
		ctx,
		slog.LevelInfo,
		"avatar deleted event received",
		observabilitylogging.AppendTraceAttrs(
			ctx,
			slog.String("avatar_id", event.AvatarID),
			slog.String("user_id", event.UserID),
			slog.String("s3_key", event.S3Key),
			slog.Int("thumbnails_count", len(event.ThumbnailS3Keys)),
		)...,
	)

	if strings.TrimSpace(event.S3Key) != "" {
		if err = p.storage.Delete(ctx, event.S3Key); err != nil {
			return fmt.Errorf("delete original avatar from s3: %w", err)
		}
	}

	for size, key := range event.ThumbnailS3Keys {
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}

		if err = p.storage.Delete(ctx, key); err != nil {
			return fmt.Errorf("delete avatar thumbnail %s from s3: %w", size, err)
		}
	}

	p.log.LogAttrs(
		ctx,
		slog.LevelInfo,
		"avatar files deleted from s3",
		observabilitylogging.AppendTraceAttrs(
			ctx,
			slog.String("avatar_id", event.AvatarID),
			slog.String("user_id", event.UserID),
		)...,
	)

	p.avatarMetrics.RecordAvatarDelete(observabilitymetrics.StatusSuccess)

	span.SetAttributes(attribute.Bool("deleted_from_s3", true))

	return nil
}
