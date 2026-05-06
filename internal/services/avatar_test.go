package services

import (
	"bytes"
	"context"
	"errors"
	"io"
	"testing"
	"time"

	"github.com/Dyuzhovsergey/gophprofile/internal/domain"
)

type fakeAvatarRepository struct {
	createCalled bool
	createInput  domain.Avatar
	createErr    error

	getByIDCalled bool
	getByIDInput  string
	getByIDAvatar domain.Avatar
	getByIDErr    error

	getLatestByUserIDCalled bool
	getLatestByUserIDInput  string
	getLatestByUserIDAvatar domain.Avatar
	getLatestByUserIDErr    error
}

func (r *fakeAvatarRepository) Create(ctx context.Context, avatar domain.Avatar) (domain.Avatar, error) {
	r.createCalled = true
	r.createInput = avatar

	if r.createErr != nil {
		return domain.Avatar{}, r.createErr
	}

	now := time.Now()
	avatar.CreatedAt = now
	avatar.UpdatedAt = now

	return avatar, nil
}

func (r *fakeAvatarRepository) GetByID(ctx context.Context, id string) (domain.Avatar, error) {
	r.getByIDCalled = true
	r.getByIDInput = id

	if r.getByIDErr != nil {
		return domain.Avatar{}, r.getByIDErr
	}

	return r.getByIDAvatar, nil
}

func (r *fakeAvatarRepository) GetLatestByUserID(ctx context.Context, userID string) (domain.Avatar, error) {
	r.getLatestByUserIDCalled = true
	r.getLatestByUserIDInput = userID

	if r.getLatestByUserIDErr != nil {
		return domain.Avatar{}, r.getLatestByUserIDErr
	}

	return r.getLatestByUserIDAvatar, nil
}

type fakeAvatarStorage struct {
	uploadCalled      bool
	uploadKey         string
	uploadContentType string
	uploadBody        []byte
	uploadErr         error

	deleteCalled bool
	deleteKey    string
	deleteErr    error

	downloadCalled      bool
	downloadKey         string
	downloadData        []byte
	downloadContentType string
	downloadErr         error
}

func (s *fakeAvatarStorage) Upload(ctx context.Context, key string, body io.Reader, contentType string) error {
	s.uploadCalled = true
	s.uploadKey = key
	s.uploadContentType = contentType

	data, err := io.ReadAll(body)
	if err != nil {
		return err
	}

	s.uploadBody = data

	return s.uploadErr
}

func (s *fakeAvatarStorage) Download(ctx context.Context, key string) ([]byte, string, error) {
	s.downloadCalled = true
	s.downloadKey = key

	if s.downloadErr != nil {
		return nil, "", s.downloadErr
	}

	return s.downloadData, s.downloadContentType, nil
}

func (s *fakeAvatarStorage) Delete(ctx context.Context, key string) error {
	s.deleteCalled = true
	s.deleteKey = key

	return s.deleteErr
}

func TestAvatarService_UploadAvatar_Success(t *testing.T) {
	repo := &fakeAvatarRepository{}
	storage := &fakeAvatarStorage{}

	service := NewAvatarService(repo, storage, DefaultMaxUploadSizeBytes)

	avatar, err := service.UploadAvatar(context.Background(), UploadAvatarInput{
		UserID:    "sergey",
		FileName:  "avatar.jpg",
		MIMEType:  "image/jpeg",
		SizeBytes: 4,
		Body:      bytes.NewBufferString("data"),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if avatar.ID == "" {
		t.Fatal("expected avatar id to be generated")
	}

	if avatar.UserID != "sergey" {
		t.Fatalf("unexpected user id: got %q, want %q", avatar.UserID, "sergey")
	}

	if avatar.FileName != "avatar.jpg" {
		t.Fatalf("unexpected file name: got %q, want %q", avatar.FileName, "avatar.jpg")
	}

	if avatar.MIMEType != "image/jpeg" {
		t.Fatalf("unexpected mime type: got %q, want %q", avatar.MIMEType, "image/jpeg")
	}

	if avatar.UploadStatus != domain.UploadStatusUploaded {
		t.Fatalf("unexpected upload status: got %q, want %q", avatar.UploadStatus, domain.UploadStatusUploaded)
	}

	if avatar.ProcessingStatus != domain.ProcessingStatusPending {
		t.Fatalf(
			"unexpected processing status: got %q, want %q",
			avatar.ProcessingStatus,
			domain.ProcessingStatusPending,
		)
	}

	if !storage.uploadCalled {
		t.Fatal("expected storage upload to be called")
	}

	if storage.uploadContentType != "image/jpeg" {
		t.Fatalf("unexpected upload content type: got %q, want %q", storage.uploadContentType, "image/jpeg")
	}

	if string(storage.uploadBody) != "data" {
		t.Fatalf("unexpected upload body: got %q, want %q", string(storage.uploadBody), "data")
	}

	if !repo.createCalled {
		t.Fatal("expected repository create to be called")
	}

	if repo.createInput.S3Key == "" {
		t.Fatal("expected s3 key to be set")
	}

	if storage.deleteCalled {
		t.Fatal("did not expect storage delete to be called")
	}
}

func TestAvatarService_UploadAvatar_MissingUserID(t *testing.T) {
	repo := &fakeAvatarRepository{}
	storage := &fakeAvatarStorage{}

	service := NewAvatarService(repo, storage, DefaultMaxUploadSizeBytes)

	_, err := service.UploadAvatar(context.Background(), UploadAvatarInput{
		UserID:    "   ",
		FileName:  "avatar.jpg",
		MIMEType:  "image/jpeg",
		SizeBytes: 4,
		Body:      bytes.NewBufferString("data"),
	})

	if !errors.Is(err, domain.ErrMissingUserID) {
		t.Fatalf("expected ErrMissingUserID, got %v", err)
	}

	if storage.uploadCalled {
		t.Fatal("did not expect storage upload to be called")
	}

	if repo.createCalled {
		t.Fatal("did not expect repository create to be called")
	}
}

func TestAvatarService_UploadAvatar_FileTooLarge(t *testing.T) {
	repo := &fakeAvatarRepository{}
	storage := &fakeAvatarStorage{}

	service := NewAvatarService(repo, storage, 3)

	_, err := service.UploadAvatar(context.Background(), UploadAvatarInput{
		UserID:    "sergey",
		FileName:  "avatar.jpg",
		MIMEType:  "image/jpeg",
		SizeBytes: 4,
		Body:      bytes.NewBufferString("data"),
	})

	if !errors.Is(err, domain.ErrFileTooLarge) {
		t.Fatalf("expected ErrFileTooLarge, got %v", err)
	}

	if storage.uploadCalled {
		t.Fatal("did not expect storage upload to be called")
	}

	if repo.createCalled {
		t.Fatal("did not expect repository create to be called")
	}
}

func TestAvatarService_UploadAvatar_InvalidMIMEType(t *testing.T) {
	repo := &fakeAvatarRepository{}
	storage := &fakeAvatarStorage{}

	service := NewAvatarService(repo, storage, DefaultMaxUploadSizeBytes)

	_, err := service.UploadAvatar(context.Background(), UploadAvatarInput{
		UserID:    "sergey",
		FileName:  "avatar.txt",
		MIMEType:  "text/plain",
		SizeBytes: 4,
		Body:      bytes.NewBufferString("data"),
	})

	if !errors.Is(err, domain.ErrInvalidFile) {
		t.Fatalf("expected ErrInvalidFile, got %v", err)
	}

	if storage.uploadCalled {
		t.Fatal("did not expect storage upload to be called")
	}

	if repo.createCalled {
		t.Fatal("did not expect repository create to be called")
	}
}

func TestAvatarService_UploadAvatar_StorageError(t *testing.T) {
	repo := &fakeAvatarRepository{}
	storage := &fakeAvatarStorage{
		uploadErr: errors.New("upload failed"),
	}

	service := NewAvatarService(repo, storage, DefaultMaxUploadSizeBytes)

	_, err := service.UploadAvatar(context.Background(), UploadAvatarInput{
		UserID:    "sergey",
		FileName:  "avatar.jpg",
		MIMEType:  "image/jpeg",
		SizeBytes: 4,
		Body:      bytes.NewBufferString("data"),
	})
	if err == nil {
		t.Fatal("expected error")
	}

	if !storage.uploadCalled {
		t.Fatal("expected storage upload to be called")
	}

	if repo.createCalled {
		t.Fatal("did not expect repository create to be called")
	}

	if storage.deleteCalled {
		t.Fatal("did not expect storage delete to be called")
	}
}

func TestAvatarService_UploadAvatar_RepositoryError_CleansUploadedFile(t *testing.T) {
	repoErr := errors.New("create failed")

	repo := &fakeAvatarRepository{
		createErr: repoErr,
	}
	storage := &fakeAvatarStorage{}

	service := NewAvatarService(repo, storage, DefaultMaxUploadSizeBytes)

	_, err := service.UploadAvatar(context.Background(), UploadAvatarInput{
		UserID:    "sergey",
		FileName:  "avatar.jpg",
		MIMEType:  "image/jpeg",
		SizeBytes: 4,
		Body:      bytes.NewBufferString("data"),
	})

	if !errors.Is(err, repoErr) {
		t.Fatalf("expected repository error, got %v", err)
	}

	if !storage.uploadCalled {
		t.Fatal("expected storage upload to be called")
	}

	if !repo.createCalled {
		t.Fatal("expected repository create to be called")
	}

	if !storage.deleteCalled {
		t.Fatal("expected storage delete to be called")
	}

	if storage.deleteKey != storage.uploadKey {
		t.Fatalf("unexpected delete key: got %q, want %q", storage.deleteKey, storage.uploadKey)
	}
}

func TestAvatarService_GetAvatarByID_Success(t *testing.T) {
	repo := &fakeAvatarRepository{
		getByIDAvatar: domain.Avatar{
			ID:       "avatar-id",
			UserID:   "sergey",
			MIMEType: "image/jpeg",
			S3Key:    "originals/avatar-id/avatar.jpg",
		},
	}

	storage := &fakeAvatarStorage{
		downloadData:        []byte("image-data"),
		downloadContentType: "image/jpeg",
	}

	service := NewAvatarService(repo, storage, DefaultMaxUploadSizeBytes)

	result, err := service.GetAvatarByID(context.Background(), "avatar-id")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !repo.getByIDCalled {
		t.Fatal("expected repository GetByID to be called")
	}

	if repo.getByIDInput != "avatar-id" {
		t.Fatalf("unexpected avatar id: got %q, want %q", repo.getByIDInput, "avatar-id")
	}

	if !storage.downloadCalled {
		t.Fatal("expected storage Download to be called")
	}

	if storage.downloadKey != "originals/avatar-id/avatar.jpg" {
		t.Fatalf("unexpected download key: got %q", storage.downloadKey)
	}

	if string(result.Data) != "image-data" {
		t.Fatalf("unexpected data: got %q, want %q", string(result.Data), "image-data")
	}

	if result.ContentType != "image/jpeg" {
		t.Fatalf("unexpected content type: got %q, want %q", result.ContentType, "image/jpeg")
	}
}

func TestAvatarService_GetAvatarByID_NotFound(t *testing.T) {
	repo := &fakeAvatarRepository{
		getByIDErr: domain.ErrAvatarNotFound,
	}
	storage := &fakeAvatarStorage{}

	service := NewAvatarService(repo, storage, DefaultMaxUploadSizeBytes)

	_, err := service.GetAvatarByID(context.Background(), "avatar-id")
	if !errors.Is(err, domain.ErrAvatarNotFound) {
		t.Fatalf("expected ErrAvatarNotFound, got %v", err)
	}

	if storage.downloadCalled {
		t.Fatal("did not expect storage Download to be called")
	}
}

func TestAvatarService_GetAvatarByID_EmptyID(t *testing.T) {
	repo := &fakeAvatarRepository{}
	storage := &fakeAvatarStorage{}

	service := NewAvatarService(repo, storage, DefaultMaxUploadSizeBytes)

	_, err := service.GetAvatarByID(context.Background(), "   ")
	if !errors.Is(err, domain.ErrAvatarNotFound) {
		t.Fatalf("expected ErrAvatarNotFound, got %v", err)
	}

	if repo.getByIDCalled {
		t.Fatal("did not expect repository GetByID to be called")
	}

	if storage.downloadCalled {
		t.Fatal("did not expect storage Download to be called")
	}
}

func TestAvatarService_GetAvatarByID_StorageError(t *testing.T) {
	repo := &fakeAvatarRepository{
		getByIDAvatar: domain.Avatar{
			ID:       "avatar-id",
			UserID:   "sergey",
			MIMEType: "image/jpeg",
			S3Key:    "originals/avatar-id/avatar.jpg",
		},
	}

	storage := &fakeAvatarStorage{
		downloadErr: errors.New("download failed"),
	}

	service := NewAvatarService(repo, storage, DefaultMaxUploadSizeBytes)

	_, err := service.GetAvatarByID(context.Background(), "avatar-id")
	if err == nil {
		t.Fatal("expected error")
	}

	if !storage.downloadCalled {
		t.Fatal("expected storage Download to be called")
	}
}

func TestAvatarService_GetAvatarByID_UsesAvatarMIMETypeWhenStorageContentTypeEmpty(t *testing.T) {
	repo := &fakeAvatarRepository{
		getByIDAvatar: domain.Avatar{
			ID:       "avatar-id",
			UserID:   "sergey",
			MIMEType: "image/png",
			S3Key:    "originals/avatar-id/avatar.png",
		},
	}

	storage := &fakeAvatarStorage{
		downloadData: []byte("image-data"),
	}

	service := NewAvatarService(repo, storage, DefaultMaxUploadSizeBytes)

	result, err := service.GetAvatarByID(context.Background(), "avatar-id")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.ContentType != "image/png" {
		t.Fatalf("unexpected content type: got %q, want %q", result.ContentType, "image/png")
	}
}

func TestAvatarService_GetCurrentAvatarByUserID_Success(t *testing.T) {
	repo := &fakeAvatarRepository{
		getLatestByUserIDAvatar: domain.Avatar{
			ID:       "avatar-id",
			UserID:   "sergey",
			MIMEType: "image/jpeg",
			S3Key:    "originals/avatar-id/avatar.jpg",
		},
	}

	storage := &fakeAvatarStorage{
		downloadData:        []byte("image-data"),
		downloadContentType: "image/jpeg",
	}

	service := NewAvatarService(repo, storage, DefaultMaxUploadSizeBytes)

	result, err := service.GetCurrentAvatarByUserID(context.Background(), "sergey")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !repo.getLatestByUserIDCalled {
		t.Fatal("expected repository GetLatestByUserID to be called")
	}

	if repo.getLatestByUserIDInput != "sergey" {
		t.Fatalf("unexpected user id: got %q, want %q", repo.getLatestByUserIDInput, "sergey")
	}

	if !storage.downloadCalled {
		t.Fatal("expected storage Download to be called")
	}

	if storage.downloadKey != "originals/avatar-id/avatar.jpg" {
		t.Fatalf("unexpected download key: got %q", storage.downloadKey)
	}

	if string(result.Data) != "image-data" {
		t.Fatalf("unexpected data: got %q, want %q", string(result.Data), "image-data")
	}

	if result.ContentType != "image/jpeg" {
		t.Fatalf("unexpected content type: got %q, want %q", result.ContentType, "image/jpeg")
	}
}

func TestAvatarService_GetCurrentAvatarByUserID_EmptyUserID(t *testing.T) {
	repo := &fakeAvatarRepository{}
	storage := &fakeAvatarStorage{}

	service := NewAvatarService(repo, storage, DefaultMaxUploadSizeBytes)

	_, err := service.GetCurrentAvatarByUserID(context.Background(), "   ")
	if !errors.Is(err, domain.ErrMissingUserID) {
		t.Fatalf("expected ErrMissingUserID, got %v", err)
	}

	if repo.getLatestByUserIDCalled {
		t.Fatal("did not expect repository GetLatestByUserID to be called")
	}

	if storage.downloadCalled {
		t.Fatal("did not expect storage Download to be called")
	}
}

func TestAvatarService_GetCurrentAvatarByUserID_NotFound(t *testing.T) {
	repo := &fakeAvatarRepository{
		getLatestByUserIDErr: domain.ErrAvatarNotFound,
	}
	storage := &fakeAvatarStorage{}

	service := NewAvatarService(repo, storage, DefaultMaxUploadSizeBytes)

	_, err := service.GetCurrentAvatarByUserID(context.Background(), "sergey")
	if !errors.Is(err, domain.ErrAvatarNotFound) {
		t.Fatalf("expected ErrAvatarNotFound, got %v", err)
	}

	if storage.downloadCalled {
		t.Fatal("did not expect storage Download to be called")
	}
}

func TestAvatarService_GetAvatarMetadata_Success(t *testing.T) {
	repo := &fakeAvatarRepository{
		getByIDAvatar: domain.Avatar{
			ID:        "avatar-id",
			UserID:    "sergey",
			FileName:  "avatar.jpg",
			MIMEType:  "image/jpeg",
			SizeBytes: 1024,
			Width:     800,
			Height:    600,
			S3Key:     "originals/avatar-id/avatar.jpg",
		},
	}

	storage := &fakeAvatarStorage{}
	service := NewAvatarService(repo, storage, DefaultMaxUploadSizeBytes)

	avatar, err := service.GetAvatarMetadata(context.Background(), "avatar-id")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !repo.getByIDCalled {
		t.Fatal("expected repository GetByID to be called")
	}

	if repo.getByIDInput != "avatar-id" {
		t.Fatalf("unexpected avatar id: got %q, want %q", repo.getByIDInput, "avatar-id")
	}

	if avatar.ID != "avatar-id" {
		t.Fatalf("unexpected avatar id: got %q, want %q", avatar.ID, "avatar-id")
	}

	if storage.downloadCalled {
		t.Fatal("did not expect storage Download to be called")
	}
}

func TestAvatarService_GetAvatarMetadata_EmptyID(t *testing.T) {
	repo := &fakeAvatarRepository{}
	storage := &fakeAvatarStorage{}

	service := NewAvatarService(repo, storage, DefaultMaxUploadSizeBytes)

	_, err := service.GetAvatarMetadata(context.Background(), "   ")
	if !errors.Is(err, domain.ErrAvatarNotFound) {
		t.Fatalf("expected ErrAvatarNotFound, got %v", err)
	}

	if repo.getByIDCalled {
		t.Fatal("did not expect repository GetByID to be called")
	}
}

func TestAvatarService_GetAvatarMetadata_NotFound(t *testing.T) {
	repo := &fakeAvatarRepository{
		getByIDErr: domain.ErrAvatarNotFound,
	}
	storage := &fakeAvatarStorage{}

	service := NewAvatarService(repo, storage, DefaultMaxUploadSizeBytes)

	_, err := service.GetAvatarMetadata(context.Background(), "avatar-id")
	if !errors.Is(err, domain.ErrAvatarNotFound) {
		t.Fatalf("expected ErrAvatarNotFound, got %v", err)
	}

	if storage.downloadCalled {
		t.Fatal("did not expect storage Download to be called")
	}
}

func TestAvatarService_GetAvatarMetadata_Deleted(t *testing.T) {
	deletedAt := time.Now()

	repo := &fakeAvatarRepository{
		getByIDAvatar: domain.Avatar{
			ID:        "avatar-id",
			DeletedAt: &deletedAt,
		},
	}
	storage := &fakeAvatarStorage{}

	service := NewAvatarService(repo, storage, DefaultMaxUploadSizeBytes)

	_, err := service.GetAvatarMetadata(context.Background(), "avatar-id")
	if !errors.Is(err, domain.ErrAvatarDeleted) {
		t.Fatalf("expected ErrAvatarDeleted, got %v", err)
	}
}
