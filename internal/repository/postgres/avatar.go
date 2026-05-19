package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/Dyuzhovsergey/gophprofile/internal/domain"
	observabilitytracing "github.com/Dyuzhovsergey/gophprofile/internal/observability/tracing"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.opentelemetry.io/otel/attribute"
)

const avatarColumns = `
	id::text,
	user_id,
	file_name,
	mime_type,
	size_bytes,
	width,
	height,
	s3_key,
	thumbnail_s3_keys,
	upload_status,
	processing_status,
	created_at,
	updated_at,
	deleted_at
`

const (
	postgresSystem = "postgresql"
	avatarsTable   = "avatars"
)

// postgresAvatarAttrs возвращает общие атрибуты для PostgreSQL spans.
func postgresAvatarAttrs(operation string, attrs ...attribute.KeyValue) []attribute.KeyValue {
	result := make([]attribute.KeyValue, 0, len(attrs)+3)

	result = append(result,
		attribute.String("db.system", postgresSystem),
		attribute.String("db.operation", operation),
		attribute.String("db.table", avatarsTable),
	)

	result = append(result, attrs...)

	return result
}

// AvatarRepository работает с таблицей avatars в PostgreSQL.
type AvatarRepository struct {
	db *pgxpool.Pool
}

// NewAvatarRepository создаёт repository для работы с аватарками.
func NewAvatarRepository(db *pgxpool.Pool) *AvatarRepository {
	return &AvatarRepository{
		db: db,
	}
}

// Create создаёт запись аватарки и возвращает её с данными, заполненными базой.
func (r *AvatarRepository) Create(ctx context.Context, avatar domain.Avatar) (createdAvatar domain.Avatar, err error) {
	ctx, span := observabilitytracing.StartSpan(
		ctx,
		"postgres.avatar.create",
		postgresAvatarAttrs(
			"insert",
			attribute.String("user_id", avatar.UserID),
			attribute.String("file_name", avatar.FileName),
			attribute.String("mime_type", avatar.MIMEType),
			attribute.Int64("file_size", avatar.SizeBytes),
		)...,
	)
	defer func() {
		observabilitytracing.RecordError(span, err)
		span.End()
	}()

	createdAvatar, err = createAvatar(ctx, r.db, avatar)
	if err != nil {
		return domain.Avatar{}, err
	}

	span.SetAttributes(attribute.String("avatar_id", createdAvatar.ID))

	return createdAvatar, nil
}

// CreateWithUploadEvent создаёт аватарку и outbox-событие avatar.uploaded в одной транзакции.
func (r *AvatarRepository) CreateWithUploadEvent(
	ctx context.Context,
	avatar domain.Avatar,
) (createdAvatar domain.Avatar, err error) {
	ctx, span := observabilitytracing.StartSpan(
		ctx,
		"postgres.avatar.create_with_upload_event",
		postgresAvatarAttrs(
			"transaction",
			attribute.String("user_id", avatar.UserID),
			attribute.String("file_name", avatar.FileName),
			attribute.String("mime_type", avatar.MIMEType),
			attribute.Int64("file_size", avatar.SizeBytes),
			attribute.String("outbox_event_type", string(domain.OutboxEventTypeAvatarUploaded)),
		)...,
	)
	defer func() {
		observabilitytracing.RecordError(span, err)
		span.End()
	}()

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return domain.Avatar{}, fmt.Errorf("begin create avatar transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	createdAvatar, err = createAvatar(ctx, tx, avatar)
	if err != nil {
		return domain.Avatar{}, err
	}

	span.SetAttributes(attribute.String("avatar_id", createdAvatar.ID))

	payload, err := json.Marshal(domain.AvatarUploadEvent{
		AvatarID: createdAvatar.ID,
		UserID:   createdAvatar.UserID,
		S3Key:    createdAvatar.S3Key,
	})
	if err != nil {
		return domain.Avatar{}, fmt.Errorf("marshal avatar uploaded event: %w", err)
	}

	if _, err := createOutboxEvent(ctx, tx, domain.OutboxEvent{
		EventType: domain.OutboxEventTypeAvatarUploaded,
		Payload:   payload,
	}); err != nil {
		return domain.Avatar{}, fmt.Errorf("create avatar uploaded outbox event: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return domain.Avatar{}, fmt.Errorf("commit create avatar transaction: %w", err)
	}

	return createdAvatar, nil
}

// createAvatar создаёт запись аватарки через переданный executor.
// Executor может быть обычным pool или транзакцией.
func createAvatar(
	ctx context.Context,
	db queryRower,
	avatar domain.Avatar,
) (domain.Avatar, error) {
	avatar = prepareAvatarForCreate(avatar)

	if !avatar.UploadStatus.IsValid() {
		return domain.Avatar{}, domain.ErrInvalidStatus
	}

	if !avatar.ProcessingStatus.IsValid() {
		return domain.Avatar{}, domain.ErrInvalidStatus
	}

	thumbnailS3Keys, err := encodeThumbnailS3Keys(avatar.ThumbnailS3Keys)
	if err != nil {
		return domain.Avatar{}, fmt.Errorf("encode thumbnail s3 keys: %w", err)
	}

	query := `
		INSERT INTO avatars (
			id,
			user_id,
			file_name,
			mime_type,
			size_bytes,
			width,
			height,
			s3_key,
			thumbnail_s3_keys,
			upload_status,
			processing_status
		)
		VALUES (
			COALESCE(NULLIF($1, '')::uuid, gen_random_uuid()),
			$2,
			$3,
			$4,
			$5,
			$6,
			$7,
			$8,
			$9,
			$10,
			$11
		)
		RETURNING ` + avatarColumns

	createdAvatar, err := scanAvatar(db.QueryRow(
		ctx,
		query,
		avatar.ID,
		avatar.UserID,
		avatar.FileName,
		avatar.MIMEType,
		avatar.SizeBytes,
		avatar.Width,
		avatar.Height,
		avatar.S3Key,
		thumbnailS3Keys,
		string(avatar.UploadStatus),
		string(avatar.ProcessingStatus),
	))
	if err != nil {
		return domain.Avatar{}, fmt.Errorf("create avatar: %w", err)
	}

	return createdAvatar, nil
}

// GetByID возвращает аватарку по id, включая мягко удалённые записи.
func (r *AvatarRepository) GetByID(ctx context.Context, id string) (avatar domain.Avatar, err error) {
	ctx, span := observabilitytracing.StartSpan(
		ctx,
		"postgres.avatar.get_by_id",
		postgresAvatarAttrs(
			"select",
			attribute.String("avatar_id", id),
		)...,
	)
	defer func() {
		observabilitytracing.RecordError(span, err)
		span.End()
	}()

	query := `
		SELECT ` + avatarColumns + `
		FROM avatars
		WHERE id = $1
	`

	avatar, err = scanAvatar(r.db.QueryRow(ctx, query, id))
	if err != nil {
		return domain.Avatar{}, mapAvatarScanError(err)
	}

	span.SetAttributes(
		attribute.String("user_id", avatar.UserID),
		attribute.String("processing_status", string(avatar.ProcessingStatus)),
	)

	return avatar, nil
}

// GetLatestByUserID возвращает последнюю неудалённую аватарку пользователя.
func (r *AvatarRepository) GetLatestByUserID(ctx context.Context, userID string) (avatar domain.Avatar, err error) {
	ctx, span := observabilitytracing.StartSpan(
		ctx,
		"postgres.avatar.get_latest_by_user_id",
		postgresAvatarAttrs(
			"select",
			attribute.String("user_id", userID),
		)...,
	)
	defer func() {
		observabilitytracing.RecordError(span, err)
		span.End()
	}()

	query := `
		SELECT ` + avatarColumns + `
		FROM avatars
		WHERE user_id = $1
			AND deleted_at IS NULL
		ORDER BY created_at DESC
		LIMIT 1
	`

	avatar, err = scanAvatar(r.db.QueryRow(ctx, query, userID))
	if err != nil {
		return domain.Avatar{}, mapAvatarScanError(err)
	}

	span.SetAttributes(
		attribute.String("avatar_id", avatar.ID),
		attribute.String("processing_status", string(avatar.ProcessingStatus)),
	)

	return avatar, nil
}

// ListByUserID возвращает список неудалённых аватарок пользователя.
func (r *AvatarRepository) ListByUserID(ctx context.Context, userID string) (avatars []domain.Avatar, err error) {
	ctx, span := observabilitytracing.StartSpan(
		ctx,
		"postgres.avatar.list_by_user_id",
		postgresAvatarAttrs(
			"select",
			attribute.String("user_id", userID),
		)...,
	)
	defer func() {
		observabilitytracing.RecordError(span, err)
		span.End()
	}()

	query := `
		SELECT ` + avatarColumns + `
		FROM avatars
		WHERE user_id = $1
			AND deleted_at IS NULL
		ORDER BY created_at DESC
	`

	rows, err := r.db.Query(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("query avatars by user id: %w", err)
	}
	defer rows.Close()

	avatars = make([]domain.Avatar, 0)

	for rows.Next() {
		avatar, err := scanAvatar(rows)
		if err != nil {
			return nil, fmt.Errorf("scan avatar: %w", err)
		}

		avatars = append(avatars, avatar)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate avatars: %w", err)
	}

	span.SetAttributes(attribute.Int("avatars_count", len(avatars)))

	return avatars, nil
}

// SoftDelete мягко удаляет аватарку и возвращает удалённую запись.
func (r *AvatarRepository) SoftDelete(ctx context.Context, id string) (avatar domain.Avatar, err error) {
	ctx, span := observabilitytracing.StartSpan(
		ctx,
		"postgres.avatar.soft_delete",
		postgresAvatarAttrs(
			"update",
			attribute.String("avatar_id", id),
		)...,
	)
	defer func() {
		observabilitytracing.RecordError(span, err)
		span.End()
	}()

	avatar, err = softDeleteAvatar(ctx, r.db, id)
	if err != nil {
		return domain.Avatar{}, err
	}

	span.SetAttributes(
		attribute.String("user_id", avatar.UserID),
		attribute.Bool("deleted", true),
	)

	return avatar, nil
}

// SoftDeleteWithDeleteEvent мягко удаляет аватарку и создаёт outbox-событие avatar.deleted в одной транзакции.
func (r *AvatarRepository) SoftDeleteWithDeleteEvent(ctx context.Context, id string) (deletedAvatar domain.Avatar, err error) {
	ctx, span := observabilitytracing.StartSpan(
		ctx,
		"postgres.avatar.soft_delete_with_delete_event",
		postgresAvatarAttrs(
			"transaction",
			attribute.String("avatar_id", id),
			attribute.String("outbox_event_type", string(domain.OutboxEventTypeAvatarDeleted)),
		)...,
	)
	defer func() {
		observabilitytracing.RecordError(span, err)
		span.End()
	}()

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return domain.Avatar{}, fmt.Errorf("begin soft delete avatar transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	deletedAvatar, err = softDeleteAvatar(ctx, tx, id)
	if err != nil {
		return domain.Avatar{}, err
	}

	span.SetAttributes(
		attribute.String("user_id", deletedAvatar.UserID),
		attribute.Bool("deleted", true),
	)

	payload, err := json.Marshal(domain.AvatarDeletedEvent{
		AvatarID:        deletedAvatar.ID,
		UserID:          deletedAvatar.UserID,
		S3Key:           deletedAvatar.S3Key,
		ThumbnailS3Keys: deletedAvatar.ThumbnailS3Keys,
	})
	if err != nil {
		return domain.Avatar{}, fmt.Errorf("marshal avatar deleted event: %w", err)
	}

	if _, err := createOutboxEvent(ctx, tx, domain.OutboxEvent{
		EventType: domain.OutboxEventTypeAvatarDeleted,
		Payload:   payload,
	}); err != nil {
		return domain.Avatar{}, fmt.Errorf("create avatar deleted outbox event: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return domain.Avatar{}, fmt.Errorf("commit soft delete avatar transaction: %w", err)
	}

	return deletedAvatar, nil
}

// softDeleteAvatar мягко удаляет аватарку через переданный executor.
func softDeleteAvatar(
	ctx context.Context,
	db queryRower,
	id string,
) (domain.Avatar, error) {
	query := `
		UPDATE avatars
		SET
			deleted_at = NOW(),
			updated_at = NOW()
		WHERE id = $1
			AND deleted_at IS NULL
		RETURNING ` + avatarColumns

	avatar, err := scanAvatar(db.QueryRow(ctx, query, id))
	if err != nil {
		return domain.Avatar{}, mapAvatarScanError(err)
	}

	return avatar, nil
}

// UpdateProcessingStatus обновляет статус фоновой обработки аватарки.
func (r *AvatarRepository) UpdateProcessingStatus(
	ctx context.Context,
	id string,
	status domain.ProcessingStatus,
) (avatar domain.Avatar, err error) {
	ctx, span := observabilitytracing.StartSpan(
		ctx,
		"postgres.avatar.update_processing_status",
		postgresAvatarAttrs(
			"update",
			attribute.String("avatar_id", id),
			attribute.String("processing_status", string(status)),
		)...,
	)
	defer func() {
		observabilitytracing.RecordError(span, err)
		span.End()
	}()

	if !status.IsValid() {
		return domain.Avatar{}, domain.ErrInvalidStatus
	}

	query := `
		UPDATE avatars
		SET
			processing_status = $2,
			updated_at = NOW()
		WHERE id = $1
			AND deleted_at IS NULL
		RETURNING ` + avatarColumns

	avatar, err = scanAvatar(r.db.QueryRow(ctx, query, id, string(status)))
	if err != nil {
		return domain.Avatar{}, mapAvatarScanError(err)
	}

	span.SetAttributes(attribute.String("user_id", avatar.UserID))

	return avatar, nil
}

// UpdateThumbnails обновляет S3-ключи миниатюр аватарки.
func (r *AvatarRepository) UpdateThumbnails(
	ctx context.Context,
	id string,
	thumbnails map[domain.ThumbnailSize]string,
) (avatar domain.Avatar, err error) {
	ctx, span := observabilitytracing.StartSpan(
		ctx,
		"postgres.avatar.update_thumbnails",
		postgresAvatarAttrs(
			"update",
			attribute.String("avatar_id", id),
			attribute.Int("thumbnails_count", len(thumbnails)),
		)...,
	)
	defer func() {
		observabilitytracing.RecordError(span, err)
		span.End()
	}()

	thumbnailS3Keys, err := encodeThumbnailS3Keys(thumbnails)
	if err != nil {
		return domain.Avatar{}, fmt.Errorf("encode thumbnail s3 keys: %w", err)
	}

	query := `
		UPDATE avatars
		SET
			thumbnail_s3_keys = $2,
			updated_at = NOW()
		WHERE id = $1
			AND deleted_at IS NULL
		RETURNING ` + avatarColumns

	avatar, err = scanAvatar(r.db.QueryRow(ctx, query, id, thumbnailS3Keys))
	if err != nil {
		return domain.Avatar{}, mapAvatarScanError(err)
	}

	span.SetAttributes(attribute.String("user_id", avatar.UserID))

	return avatar, nil
}

// UpdateProcessingResult обновляет результат обработки изображения.
func (r *AvatarRepository) UpdateProcessingResult(
	ctx context.Context,
	id string,
	width int,
	height int,
	thumbnails map[domain.ThumbnailSize]string,
	status domain.ProcessingStatus,
) (avatar domain.Avatar, err error) {
	ctx, span := observabilitytracing.StartSpan(
		ctx,
		"postgres.avatar.update_processing_result",
		postgresAvatarAttrs(
			"update",
			attribute.String("avatar_id", id),
			attribute.Int("width", width),
			attribute.Int("height", height),
			attribute.Int("thumbnails_count", len(thumbnails)),
			attribute.String("processing_status", string(status)),
		)...,
	)
	defer func() {
		observabilitytracing.RecordError(span, err)
		span.End()
	}()

	if !status.IsValid() {
		return domain.Avatar{}, domain.ErrInvalidStatus
	}

	thumbnailS3Keys, err := encodeThumbnailS3Keys(thumbnails)
	if err != nil {
		return domain.Avatar{}, fmt.Errorf("encode thumbnail s3 keys: %w", err)
	}

	query := `
		UPDATE avatars
		SET
			width = $2,
			height = $3,
			thumbnail_s3_keys = $4,
			processing_status = $5,
			updated_at = NOW()
		WHERE id = $1
			AND deleted_at IS NULL
		RETURNING ` + avatarColumns

	avatar, err = scanAvatar(
		r.db.QueryRow(
			ctx,
			query,
			id,
			width,
			height,
			thumbnailS3Keys,
			string(status),
		),
	)
	if err != nil {
		return domain.Avatar{}, mapAvatarScanError(err)
	}

	span.SetAttributes(attribute.String("user_id", avatar.UserID))

	return avatar, nil
}

// avatarScanner описывает общий интерфейс для pgx.Row и pgx.Rows.
type avatarScanner interface {
	Scan(dest ...any) error
}

// scanAvatar читает строку PostgreSQL и преобразует её в domain.Avatar.
func scanAvatar(row avatarScanner) (domain.Avatar, error) {
	var avatar domain.Avatar
	var thumbnailS3KeysJSON []byte
	var uploadStatus string
	var processingStatus string
	var deletedAt pgtype.Timestamptz

	err := row.Scan(
		&avatar.ID,
		&avatar.UserID,
		&avatar.FileName,
		&avatar.MIMEType,
		&avatar.SizeBytes,
		&avatar.Width,
		&avatar.Height,
		&avatar.S3Key,
		&thumbnailS3KeysJSON,
		&uploadStatus,
		&processingStatus,
		&avatar.CreatedAt,
		&avatar.UpdatedAt,
		&deletedAt,
	)
	if err != nil {
		return domain.Avatar{}, err
	}

	thumbnailS3Keys, err := decodeThumbnailS3Keys(thumbnailS3KeysJSON)
	if err != nil {
		return domain.Avatar{}, fmt.Errorf("decode thumbnail s3 keys: %w", err)
	}

	avatar.ThumbnailS3Keys = thumbnailS3Keys
	avatar.UploadStatus = domain.UploadStatus(uploadStatus)
	avatar.ProcessingStatus = domain.ProcessingStatus(processingStatus)

	if deletedAt.Valid {
		deletedTime := deletedAt.Time
		avatar.DeletedAt = &deletedTime
	}

	return avatar, nil
}

// prepareAvatarForCreate выставляет значения по умолчанию перед созданием записи.
func prepareAvatarForCreate(avatar domain.Avatar) domain.Avatar {
	if avatar.UploadStatus == "" {
		avatar.UploadStatus = domain.UploadStatusUploaded
	}

	if avatar.ProcessingStatus == "" {
		avatar.ProcessingStatus = domain.ProcessingStatusPending
	}

	if avatar.ThumbnailS3Keys == nil {
		avatar.ThumbnailS3Keys = make(map[domain.ThumbnailSize]string)
	}

	return avatar
}

// encodeThumbnailS3Keys преобразует map с typed-ключами в JSONB для PostgreSQL.
func encodeThumbnailS3Keys(thumbnails map[domain.ThumbnailSize]string) ([]byte, error) {
	if thumbnails == nil {
		thumbnails = make(map[domain.ThumbnailSize]string)
	}

	raw := make(map[string]string, len(thumbnails))
	for size, key := range thumbnails {
		raw[string(size)] = key
	}

	data, err := json.Marshal(raw)
	if err != nil {
		return nil, fmt.Errorf("marshal thumbnails: %w", err)
	}

	return data, nil
}

// decodeThumbnailS3Keys преобразует JSONB из PostgreSQL в map с typed-ключами.
func decodeThumbnailS3Keys(data []byte) (map[domain.ThumbnailSize]string, error) {
	if len(data) == 0 {
		return make(map[domain.ThumbnailSize]string), nil
	}

	raw := make(map[string]string)
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("unmarshal thumbnails: %w", err)
	}

	thumbnails := make(map[domain.ThumbnailSize]string, len(raw))
	for size, key := range raw {
		thumbnails[domain.ThumbnailSize(size)] = key
	}

	return thumbnails, nil
}

// mapAvatarScanError преобразует ошибку БД в доменную ошибку.
func mapAvatarScanError(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ErrAvatarNotFound
	}

	return fmt.Errorf("scan avatar: %w", err)
}
