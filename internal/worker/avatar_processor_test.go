package worker

import (
	"bytes"
	"context"
	"errors"
	"io"
	"testing"
	"time"

	"github.com/Dyuzhovsergey/gophprofile/internal/domain"
	"github.com/Dyuzhovsergey/gophprofile/internal/logger"
	"github.com/Dyuzhovsergey/gophprofile/internal/services"
)

type fakeAvatarRepository struct {
	getByIDCalled bool
	getByIDInput  string
	getByIDAvatar domain.Avatar
	getByIDErr    error

	updateStatuses []domain.ProcessingStatus

	updateResultCalled     bool
	updateResultID         string
	updateResultWidth      int
	updateResultHeight     int
	updateResultThumbnails map[domain.ThumbnailSize]string
	updateResultStatus     domain.ProcessingStatus
	updateResultErr        error
}

func (r *fakeAvatarRepository) GetByID(ctx context.Context, id string) (domain.Avatar, error) {
	r.getByIDCalled = true
	r.getByIDInput = id

	if r.getByIDErr != nil {
		return domain.Avatar{}, r.getByIDErr
	}

	return r.getByIDAvatar, nil
}

func (r *fakeAvatarRepository) UpdateProcessingStatus(
	ctx context.Context,
	id string,
	status domain.ProcessingStatus,
) (domain.Avatar, error) {
	r.updateStatuses = append(r.updateStatuses, status)

	return domain.Avatar{
		ID:               id,
		ProcessingStatus: status,
	}, nil
}

func (r *fakeAvatarRepository) UpdateProcessingResult(
	ctx context.Context,
	id string,
	width int,
	height int,
	thumbnails map[domain.ThumbnailSize]string,
	status domain.ProcessingStatus,
) (domain.Avatar, error) {
	r.updateResultCalled = true
	r.updateResultID = id
	r.updateResultWidth = width
	r.updateResultHeight = height
	r.updateResultThumbnails = thumbnails
	r.updateResultStatus = status

	if r.updateResultErr != nil {
		return domain.Avatar{}, r.updateResultErr
	}

	return domain.Avatar{
		ID:               id,
		Width:            width,
		Height:           height,
		ThumbnailS3Keys:  thumbnails,
		ProcessingStatus: status,
	}, nil
}

type fakeAvatarStorage struct {
	downloadCalled bool
	downloadKey    string
	downloadData   []byte
	downloadErr    error

	uploadKeys []string
	uploadErr  error
	deleteKeys []string
	deleteErr  error
}

func (s *fakeAvatarStorage) Download(ctx context.Context, key string) ([]byte, string, error) {
	s.downloadCalled = true
	s.downloadKey = key

	if s.downloadErr != nil {
		return nil, "", s.downloadErr
	}

	return s.downloadData, "image/jpeg", nil
}

func (s *fakeAvatarStorage) Upload(ctx context.Context, key string, body io.Reader, contentType string) error {
	s.uploadKeys = append(s.uploadKeys, key)

	if _, err := io.Copy(io.Discard, body); err != nil {
		return err
	}

	return s.uploadErr
}

func (s *fakeAvatarStorage) Delete(ctx context.Context, key string) error {
	s.deleteKeys = append(s.deleteKeys, key)

	return s.deleteErr
}

type fakeImageProcessor struct {
	processCalled bool
	processInput  []byte
	processResult services.ImageProcessResult
	processErr    error
}

func (p *fakeImageProcessor) Process(data []byte) (services.ImageProcessResult, error) {
	p.processCalled = true
	p.processInput = data

	if p.processErr != nil {
		return services.ImageProcessResult{}, p.processErr
	}

	return p.processResult, nil
}

func TestAvatarProcessor_HandleAvatarUploaded_Success(t *testing.T) {
	repo := &fakeAvatarRepository{
		getByIDAvatar: domain.Avatar{
			ID:     "avatar-id",
			UserID: "sergey",
			S3Key:  "originals/avatar-id/avatar.jpg",
		},
	}

	storage := &fakeAvatarStorage{
		downloadData: []byte("original-image"),
	}

	imageProcessor := &fakeImageProcessor{
		processResult: services.ImageProcessResult{
			Width:  800,
			Height: 600,
			Thumbnails: []services.ImageThumbnail{
				{
					Size:        domain.ThumbnailSize100,
					Data:        []byte("thumb-100"),
					ContentType: "image/jpeg",
					Extension:   ".jpg",
				},
				{
					Size:        domain.ThumbnailSize300,
					Data:        []byte("thumb-300"),
					ContentType: "image/jpeg",
					Extension:   ".jpg",
				},
			},
		},
	}

	processor := NewAvatarProcessor(logger.NewNop(), repo, storage, imageProcessor)

	err := processor.HandleAvatarUploaded(context.Background(), domain.AvatarUploadEvent{
		AvatarID: "avatar-id",
		UserID:   "sergey",
		S3Key:    "originals/avatar-id/avatar.jpg",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !repo.getByIDCalled {
		t.Fatal("expected GetByID to be called")
	}

	if repo.getByIDInput != "avatar-id" {
		t.Fatalf("unexpected avatar id: got %q, want %q", repo.getByIDInput, "avatar-id")
	}

	if !storage.downloadCalled {
		t.Fatal("expected Download to be called")
	}

	if storage.downloadKey != "originals/avatar-id/avatar.jpg" {
		t.Fatalf("unexpected download key: got %q", storage.downloadKey)
	}

	if !imageProcessor.processCalled {
		t.Fatal("expected image processor to be called")
	}

	if !bytes.Equal(imageProcessor.processInput, []byte("original-image")) {
		t.Fatal("unexpected image processor input")
	}

	if len(storage.uploadKeys) != 2 {
		t.Fatalf("unexpected upload count: got %d, want %d", len(storage.uploadKeys), 2)
	}

	if storage.uploadKeys[0] != "thumbnails/avatar-id/100x100.jpg" {
		t.Fatalf("unexpected first thumbnail key: %q", storage.uploadKeys[0])
	}

	if storage.uploadKeys[1] != "thumbnails/avatar-id/300x300.jpg" {
		t.Fatalf("unexpected second thumbnail key: %q", storage.uploadKeys[1])
	}

	if !repo.updateResultCalled {
		t.Fatal("expected UpdateProcessingResult to be called")
	}

	if repo.updateResultWidth != 800 {
		t.Fatalf("unexpected width: got %d, want %d", repo.updateResultWidth, 800)
	}

	if repo.updateResultHeight != 600 {
		t.Fatalf("unexpected height: got %d, want %d", repo.updateResultHeight, 600)
	}

	if repo.updateResultStatus != domain.ProcessingStatusCompleted {
		t.Fatalf(
			"unexpected status: got %q, want %q",
			repo.updateResultStatus,
			domain.ProcessingStatusCompleted,
		)
	}
}

func TestAvatarProcessor_HandleAvatarUploaded_DownloadErrorMarksFailed(t *testing.T) {
	repo := &fakeAvatarRepository{
		getByIDAvatar: domain.Avatar{
			ID:    "avatar-id",
			S3Key: "originals/avatar-id/avatar.jpg",
		},
	}

	storage := &fakeAvatarStorage{
		downloadErr: errors.New("download failed"),
	}

	imageProcessor := &fakeImageProcessor{}

	processor := NewAvatarProcessor(logger.NewNop(), repo, storage, imageProcessor)

	err := processor.HandleAvatarUploaded(context.Background(), domain.AvatarUploadEvent{
		AvatarID: "avatar-id",
	})
	if err == nil {
		t.Fatal("expected error")
	}

	if len(repo.updateStatuses) == 0 {
		t.Fatal("expected processing statuses to be updated")
	}

	lastStatus := repo.updateStatuses[len(repo.updateStatuses)-1]
	if lastStatus != domain.ProcessingStatusFailed {
		t.Fatalf("unexpected last status: got %q, want %q", lastStatus, domain.ProcessingStatusFailed)
	}
}

func TestAvatarProcessor_HandleAvatarUploaded_EmptyAvatarID(t *testing.T) {
	repo := &fakeAvatarRepository{}
	storage := &fakeAvatarStorage{}
	imageProcessor := &fakeImageProcessor{}

	processor := NewAvatarProcessor(logger.NewNop(), repo, storage, imageProcessor)

	err := processor.HandleAvatarUploaded(context.Background(), domain.AvatarUploadEvent{})
	if !errors.Is(err, domain.ErrAvatarNotFound) {
		t.Fatalf("expected ErrAvatarNotFound, got %v", err)
	}

	if repo.getByIDCalled {
		t.Fatal("did not expect GetByID to be called")
	}
}

func TestAvatarProcessor_HandleAvatarUploaded_AlreadyCompletedSkipsProcessing(t *testing.T) {
	repo := &fakeAvatarRepository{
		getByIDAvatar: domain.Avatar{
			ID:               "avatar-id",
			UserID:           "sergey",
			S3Key:            "originals/avatar-id/avatar.jpg",
			ProcessingStatus: domain.ProcessingStatusCompleted,
		},
	}

	storage := &fakeAvatarStorage{
		downloadData: []byte("original-image"),
	}

	imageProcessor := &fakeImageProcessor{
		processResult: services.ImageProcessResult{
			Width:  800,
			Height: 600,
			Thumbnails: []services.ImageThumbnail{
				{
					Size:        domain.ThumbnailSize100,
					Data:        []byte("thumb-100"),
					ContentType: "image/jpeg",
					Extension:   ".jpg",
				},
			},
		},
	}

	processor := NewAvatarProcessor(logger.NewNop(), repo, storage, imageProcessor)

	err := processor.HandleAvatarUploaded(context.Background(), domain.AvatarUploadEvent{
		AvatarID: "avatar-id",
		UserID:   "sergey",
		S3Key:    "originals/avatar-id/avatar.jpg",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !repo.getByIDCalled {
		t.Fatal("expected GetByID to be called")
	}

	if len(repo.updateStatuses) != 0 {
		t.Fatalf("did not expect processing status update, got %d updates", len(repo.updateStatuses))
	}

	if storage.downloadCalled {
		t.Fatal("did not expect original image download")
	}

	if imageProcessor.processCalled {
		t.Fatal("did not expect image processing")
	}

	if len(storage.uploadKeys) != 0 {
		t.Fatalf("did not expect thumbnail uploads, got %d uploads", len(storage.uploadKeys))
	}

	if repo.updateResultCalled {
		t.Fatal("did not expect processing result update")
	}
}

func TestAvatarProcessor_HandleAvatarDeleted_Success(t *testing.T) {
	repo := &fakeAvatarRepository{}
	storage := &fakeAvatarStorage{}
	imageProcessor := &fakeImageProcessor{}

	processor := NewAvatarProcessor(logger.NewNop(), repo, storage, imageProcessor)

	err := processor.HandleAvatarDeleted(context.Background(), domain.AvatarDeletedEvent{
		AvatarID: "avatar-id",
		UserID:   "sergey",
		S3Key:    "originals/avatar-id/avatar.jpg",
		ThumbnailS3Keys: map[domain.ThumbnailSize]string{
			domain.ThumbnailSize100: "thumbnails/avatar-id/100x100.jpg",
			domain.ThumbnailSize300: "thumbnails/avatar-id/300x300.jpg",
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(storage.deleteKeys) != 3 {
		t.Fatalf("unexpected delete count: got %d, want %d", len(storage.deleteKeys), 3)
	}

	wantKeys := map[string]bool{
		"originals/avatar-id/avatar.jpg":   false,
		"thumbnails/avatar-id/100x100.jpg": false,
		"thumbnails/avatar-id/300x300.jpg": false,
	}

	for _, key := range storage.deleteKeys {
		if _, ok := wantKeys[key]; !ok {
			t.Fatalf("unexpected delete key: %q", key)
		}

		wantKeys[key] = true
	}

	for key, deleted := range wantKeys {
		if !deleted {
			t.Fatalf("expected key to be deleted: %q", key)
		}
	}
}

func TestAvatarProcessor_HandleAvatarUploaded_DeletedAvatarSkipsProcessing(t *testing.T) {
	deletedAt := time.Now()

	repo := &fakeAvatarRepository{
		getByIDAvatar: domain.Avatar{
			ID:        "avatar-id",
			UserID:    "sergey",
			S3Key:     "originals/avatar-id/avatar.jpg",
			DeletedAt: &deletedAt,
		},
	}

	storage := &fakeAvatarStorage{}
	imageProcessor := &fakeImageProcessor{}

	processor := NewAvatarProcessor(logger.NewNop(), repo, storage, imageProcessor)

	err := processor.HandleAvatarUploaded(context.Background(), domain.AvatarUploadEvent{
		AvatarID: "avatar-id",
		UserID:   "sergey",
		S3Key:    "originals/avatar-id/avatar.jpg",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !repo.getByIDCalled {
		t.Fatal("expected GetByID to be called")
	}

	if len(repo.updateStatuses) != 0 {
		t.Fatalf("did not expect processing status update, got %d updates", len(repo.updateStatuses))
	}

	if storage.downloadCalled {
		t.Fatal("did not expect original image download")
	}

	if imageProcessor.processCalled {
		t.Fatal("did not expect image processing")
	}

	if repo.updateResultCalled {
		t.Fatal("did not expect processing result update")
	}
}
