package services

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/Dyuzhovsergey/gophprofile/internal/domain"
	"github.com/google/uuid"
)

// DefaultMaxUploadSizeBytes задаёт максимальный размер файла по умолчанию: 10 MB.
const DefaultMaxUploadSizeBytes int64 = 10 * 1024 * 1024

// detectContentTypeBufferSize задаёт количество байт для определения реального типа файла.
const detectContentTypeBufferSize = 512

// AvatarRepository описывает методы хранилища метаданных аватарок.
type AvatarRepository interface {
	Create(ctx context.Context, avatar domain.Avatar) (domain.Avatar, error)
	GetByID(ctx context.Context, id string) (domain.Avatar, error)
	GetLatestByUserID(ctx context.Context, userID string) (domain.Avatar, error)
	ListByUserID(ctx context.Context, userID string) ([]domain.Avatar, error)
	SoftDelete(ctx context.Context, id string) (domain.Avatar, error)
}

// AvatarStorage описывает методы файлового хранилища аватарок.
type AvatarStorage interface {
	Upload(ctx context.Context, key string, body io.Reader, contentType string) error
	Download(ctx context.Context, key string) ([]byte, string, error)
	Delete(ctx context.Context, key string) error
}

// AvatarEventPublisher описывает публикацию событий аватарок.
type AvatarEventPublisher interface {
	PublishAvatarUploaded(ctx context.Context, event domain.AvatarUploadEvent) error
	PublishAvatarDeleted(ctx context.Context, event domain.AvatarDeletedEvent) error
}

// NoopAvatarEventPublisher используется, когда публикация событий ещё не подключена.
type NoopAvatarEventPublisher struct{}

// PublishAvatarUploaded ничего не делает и всегда возвращает nil.
func (NoopAvatarEventPublisher) PublishAvatarUploaded(ctx context.Context, event domain.AvatarUploadEvent) error {
	return nil
}

// PublishAvatarDeleted ничего не делает и всегда возвращает nil.
func (NoopAvatarEventPublisher) PublishAvatarDeleted(ctx context.Context, event domain.AvatarDeletedEvent) error {
	return nil
}

// AvatarService содержит бизнес-логику управления аватарками.
type AvatarService struct {
	repo               AvatarRepository
	storage            AvatarStorage
	eventPublisher     AvatarEventPublisher
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
	return NewAvatarServiceWithPublisher(
		repo,
		storage,
		NoopAvatarEventPublisher{},
		maxUploadSizeBytes,
	)
}

// NewAvatarServiceWithPublisher создаёт сервис управления аватарками с publisher-ом событий.
func NewAvatarServiceWithPublisher(
	repo AvatarRepository,
	storage AvatarStorage,
	eventPublisher AvatarEventPublisher,
	maxUploadSizeBytes int64,
) *AvatarService {
	if maxUploadSizeBytes <= 0 {
		maxUploadSizeBytes = DefaultMaxUploadSizeBytes
	}

	if eventPublisher == nil {
		eventPublisher = NoopAvatarEventPublisher{}
	}

	return &AvatarService{
		repo:               repo,
		storage:            storage,
		eventPublisher:     eventPublisher,
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

	detectedMIMEType, body, err := detectAvatarMIMEType(input.Body)
	if err != nil {
		return domain.Avatar{}, err
	}

	if detectedMIMEType != mimeType {
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

	if err := s.storage.Upload(ctx, s3Key, body, mimeType); err != nil {
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

	event := domain.AvatarUploadEvent{
		AvatarID: createdAvatar.ID,
		UserID:   createdAvatar.UserID,
		S3Key:    createdAvatar.S3Key,
	}

	if err := s.eventPublisher.PublishAvatarUploaded(ctx, event); err != nil {
		return domain.Avatar{}, fmt.Errorf("publish avatar uploaded event: %w", err)
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

// GetAvatarThumbnailByID получает миниатюру аватарки по id и размеру.
func (s *AvatarService) GetAvatarThumbnailByID(
	ctx context.Context,
	avatarID string,
	size domain.ThumbnailSize,
) (DownloadAvatarResult, error) {
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

	if avatar.IsDeleted() {
		return DownloadAvatarResult{}, domain.ErrAvatarDeleted
	}

	thumbnailKey := strings.TrimSpace(avatar.ThumbnailS3Keys[size])
	if thumbnailKey == "" {
		return DownloadAvatarResult{}, domain.ErrThumbnailNotFound
	}

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
func (s *AvatarService) GetAvatarMetadata(ctx context.Context, avatarID string) (domain.Avatar, error) {
	avatarID = strings.TrimSpace(avatarID)
	if avatarID == "" {
		return domain.Avatar{}, domain.ErrAvatarNotFound
	}

	avatar, err := s.repo.GetByID(ctx, avatarID)
	if err != nil {
		return domain.Avatar{}, err
	}

	if avatar.IsDeleted() {
		return domain.Avatar{}, domain.ErrAvatarDeleted
	}

	return avatar, nil
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

// ListAvatarsByUserID возвращает список активных аватарок пользователя.
func (s *AvatarService) ListAvatarsByUserID(ctx context.Context, userID string) ([]domain.Avatar, error) {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return nil, domain.ErrMissingUserID
	}

	avatars, err := s.repo.ListByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}

	return avatars, nil
}

// DeleteAvatarByID мягко удаляет аватарку по id, если она принадлежит пользователю.
func (s *AvatarService) DeleteAvatarByID(
	ctx context.Context,
	avatarID string,
	userID string,
) (domain.Avatar, error) {
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

	if avatar.IsDeleted() {
		return domain.Avatar{}, domain.ErrAvatarDeleted
	}

	if !avatar.IsOwner(userID) {
		return domain.Avatar{}, domain.ErrForbidden
	}

	deletedAvatar, err := s.repo.SoftDelete(ctx, avatarID)
	if err != nil {
		return domain.Avatar{}, err
	}

	if err := s.publishAvatarDeleted(ctx, deletedAvatar); err != nil {
		return domain.Avatar{}, err
	}

	return deletedAvatar, nil
}

// DeleteCurrentAvatarByUserID мягко удаляет последнюю активную аватарку пользователя.
func (s *AvatarService) DeleteCurrentAvatarByUserID(
	ctx context.Context,
	userID string,
	actorUserID string,
) (domain.Avatar, error) {
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

	if avatar.IsDeleted() {
		return domain.Avatar{}, domain.ErrAvatarDeleted
	}

	if !avatar.IsOwner(actorUserID) {
		return domain.Avatar{}, domain.ErrForbidden
	}

	deletedAvatar, err := s.repo.SoftDelete(ctx, avatar.ID)
	if err != nil {
		return domain.Avatar{}, err
	}

	if err := s.publishAvatarDeleted(ctx, deletedAvatar); err != nil {
		return domain.Avatar{}, err
	}

	return deletedAvatar, nil
}

// publishAvatarDeleted публикует событие удаления аватарки.
func (s *AvatarService) publishAvatarDeleted(ctx context.Context, avatar domain.Avatar) error {
	event := domain.AvatarDeletedEvent{
		AvatarID:        avatar.ID,
		UserID:          avatar.UserID,
		S3Key:           avatar.S3Key,
		ThumbnailS3Keys: avatar.ThumbnailS3Keys,
	}

	if err := s.eventPublisher.PublishAvatarDeleted(ctx, event); err != nil {
		return fmt.Errorf("publish avatar deleted event: %w", err)
	}

	return nil
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
