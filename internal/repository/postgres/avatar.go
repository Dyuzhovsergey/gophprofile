package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/Dyuzhovsergey/gophprofile/internal/domain"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
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
func (r *AvatarRepository) Create(ctx context.Context, avatar domain.Avatar) (domain.Avatar, error) {
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

	createdAvatar, err := scanAvatar(r.db.QueryRow(
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

// GetByID возвращает неудалённую аватарку по id.
func (r *AvatarRepository) GetByID(ctx context.Context, id string) (domain.Avatar, error) {
	query := `
		SELECT ` + avatarColumns + `
		FROM avatars
		WHERE id = $1
			AND deleted_at IS NULL
	`

	avatar, err := scanAvatar(r.db.QueryRow(ctx, query, id))
	if err != nil {
		return domain.Avatar{}, mapAvatarScanError(err)
	}

	return avatar, nil
}

// GetLatestByUserID возвращает последнюю неудалённую аватарку пользователя.
func (r *AvatarRepository) GetLatestByUserID(ctx context.Context, userID string) (domain.Avatar, error) {
	query := `
		SELECT ` + avatarColumns + `
		FROM avatars
		WHERE user_id = $1
			AND deleted_at IS NULL
		ORDER BY created_at DESC
		LIMIT 1
	`

	avatar, err := scanAvatar(r.db.QueryRow(ctx, query, userID))
	if err != nil {
		return domain.Avatar{}, mapAvatarScanError(err)
	}

	return avatar, nil
}

// ListByUserID возвращает список неудалённых аватарок пользователя.
func (r *AvatarRepository) ListByUserID(ctx context.Context, userID string) ([]domain.Avatar, error) {
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

	avatars := make([]domain.Avatar, 0)

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

	return avatars, nil
}

// SoftDelete мягко удаляет аватарку и возвращает удалённую запись.
// Физическое удаление файлов из S3 позже будет делать worker.
func (r *AvatarRepository) SoftDelete(ctx context.Context, id string) (domain.Avatar, error) {
	query := `
		UPDATE avatars
		SET
			deleted_at = NOW(),
			updated_at = NOW()
		WHERE id = $1
			AND deleted_at IS NULL
		RETURNING ` + avatarColumns

	avatar, err := scanAvatar(r.db.QueryRow(ctx, query, id))
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
) (domain.Avatar, error) {
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

	avatar, err := scanAvatar(r.db.QueryRow(ctx, query, id, string(status)))
	if err != nil {
		return domain.Avatar{}, mapAvatarScanError(err)
	}

	return avatar, nil
}

// UpdateThumbnails обновляет S3-ключи миниатюр аватарки.
func (r *AvatarRepository) UpdateThumbnails(
	ctx context.Context,
	id string,
	thumbnails map[domain.ThumbnailSize]string,
) (domain.Avatar, error) {
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

	avatar, err := scanAvatar(r.db.QueryRow(ctx, query, id, thumbnailS3Keys))
	if err != nil {
		return domain.Avatar{}, mapAvatarScanError(err)
	}

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
