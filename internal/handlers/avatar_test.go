package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"testing"
	"time"

	"github.com/Dyuzhovsergey/gophprofile/internal/domain"
	"github.com/Dyuzhovsergey/gophprofile/internal/middleware"
	"github.com/Dyuzhovsergey/gophprofile/internal/services"
	"github.com/go-chi/chi/v5"
)

type fakeAvatarUploader struct {
	called bool
	input  services.UploadAvatarInput
	err    error

	avatar domain.Avatar

	getByIDCalled bool
	getByIDInput  string
	getByIDResult services.DownloadAvatarResult
	getByIDErr    error

	getCurrentCalled bool
	getCurrentInput  string
	getCurrentResult services.DownloadAvatarResult
	getCurrentErr    error

	getMetadataCalled bool
	getMetadataInput  string
	getMetadataAvatar domain.Avatar
	getMetadataErr    error

	listByUserIDCalled  bool
	listByUserIDInput   string
	listByUserIDAvatars []domain.Avatar
	listByUserIDErr     error

	deleteCalled   bool
	deleteAvatarID string
	deleteUserID   string
	deleteAvatar   domain.Avatar
	deleteErr      error

	getLatestByUserIDCalled bool
	getLatestByUserIDInput  string
	getLatestByUserIDAvatar domain.Avatar
	getLatestByUserIDErr    error

	softDeleteCalled bool
	softDeleteInput  string
	softDeleteAvatar domain.Avatar
	softDeleteErr    error
}

func (u *fakeAvatarUploader) UploadAvatar(ctx context.Context, input services.UploadAvatarInput) (domain.Avatar, error) {
	u.called = true
	u.input = input

	if u.err != nil {
		return domain.Avatar{}, u.err
	}

	return u.avatar, nil
}

func (u *fakeAvatarUploader) GetAvatarByID(
	ctx context.Context,
	avatarID string,
) (services.DownloadAvatarResult, error) {
	u.getByIDCalled = true
	u.getByIDInput = avatarID

	if u.getByIDErr != nil {
		return services.DownloadAvatarResult{}, u.getByIDErr
	}

	return u.getByIDResult, nil
}

func (u *fakeAvatarUploader) GetCurrentAvatarByUserID(
	ctx context.Context,
	userID string,
) (services.DownloadAvatarResult, error) {
	u.getCurrentCalled = true
	u.getCurrentInput = userID

	if u.getCurrentErr != nil {
		return services.DownloadAvatarResult{}, u.getCurrentErr
	}

	return u.getCurrentResult, nil
}

func (u *fakeAvatarUploader) GetAvatarMetadata(
	ctx context.Context,
	avatarID string,
) (domain.Avatar, error) {
	u.getMetadataCalled = true
	u.getMetadataInput = avatarID

	if u.getMetadataErr != nil {
		return domain.Avatar{}, u.getMetadataErr
	}

	return u.getMetadataAvatar, nil
}

func (u *fakeAvatarUploader) ListAvatarsByUserID(
	ctx context.Context,
	userID string,
) ([]domain.Avatar, error) {
	u.listByUserIDCalled = true
	u.listByUserIDInput = userID

	if u.listByUserIDErr != nil {
		return nil, u.listByUserIDErr
	}

	return u.listByUserIDAvatars, nil
}

func (u *fakeAvatarUploader) DeleteAvatarByID(
	ctx context.Context,
	avatarID string,
	userID string,
) (domain.Avatar, error) {
	u.deleteCalled = true
	u.deleteAvatarID = avatarID
	u.deleteUserID = userID

	if u.deleteErr != nil {
		return domain.Avatar{}, u.deleteErr
	}

	return u.deleteAvatar, nil
}

func TestAvatarHandler_Upload_Success(t *testing.T) {
	createdAt := time.Date(2026, 5, 4, 12, 0, 0, 0, time.UTC)

	uploader := &fakeAvatarUploader{
		avatar: domain.Avatar{
			ID:               "avatar-id",
			UserID:           "sergey",
			ProcessingStatus: domain.ProcessingStatusPending,
			CreatedAt:        createdAt,
		},
	}

	handler := NewAvatarHandler(uploader, services.DefaultMaxUploadSizeBytes)

	req := newMultipartUploadRequest(t, "avatar.jpg", "image/jpeg", []byte("data"))
	req.Header.Set("X-User-ID", "sergey")

	rec := httptest.NewRecorder()

	wrappedHandler := middleware.RequireUserID(http.HandlerFunc(handler.Upload))
	wrappedHandler.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("unexpected status code: got %d, want %d", rec.Code, http.StatusCreated)
	}

	if !uploader.called {
		t.Fatal("expected uploader to be called")
	}

	if uploader.input.UserID != "sergey" {
		t.Fatalf("unexpected user id: got %q, want %q", uploader.input.UserID, "sergey")
	}

	if uploader.input.FileName != "avatar.jpg" {
		t.Fatalf("unexpected file name: got %q, want %q", uploader.input.FileName, "avatar.jpg")
	}

	if uploader.input.MIMEType != "image/jpeg" {
		t.Fatalf("unexpected mime type: got %q, want %q", uploader.input.MIMEType, "image/jpeg")
	}

	if uploader.input.SizeBytes != int64(len("data")) {
		t.Fatalf("unexpected size: got %d, want %d", uploader.input.SizeBytes, len("data"))
	}

	var response UploadAvatarResponse
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if response.ID != "avatar-id" {
		t.Fatalf("unexpected avatar id: got %q, want %q", response.ID, "avatar-id")
	}

	if response.URL != "/api/v1/avatars/avatar-id" {
		t.Fatalf("unexpected url: got %q", response.URL)
	}
}

func TestAvatarHandler_Upload_MissingUserID(t *testing.T) {
	uploader := &fakeAvatarUploader{}
	handler := NewAvatarHandler(uploader, services.DefaultMaxUploadSizeBytes)

	req := newMultipartUploadRequest(t, "avatar.jpg", "image/jpeg", []byte("data"))
	rec := httptest.NewRecorder()

	wrappedHandler := middleware.RequireUserID(http.HandlerFunc(handler.Upload))
	wrappedHandler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("unexpected status code: got %d, want %d", rec.Code, http.StatusBadRequest)
	}

	if uploader.called {
		t.Fatal("did not expect uploader to be called")
	}
}

func TestAvatarHandler_Upload_MissingFile(t *testing.T) {
	uploader := &fakeAvatarUploader{}
	handler := NewAvatarHandler(uploader, services.DefaultMaxUploadSizeBytes)

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	if err := writer.Close(); err != nil {
		t.Fatalf("failed to close multipart writer: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/avatars", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("X-User-ID", "sergey")

	rec := httptest.NewRecorder()

	wrappedHandler := middleware.RequireUserID(http.HandlerFunc(handler.Upload))
	wrappedHandler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("unexpected status code: got %d, want %d", rec.Code, http.StatusBadRequest)
	}

	if uploader.called {
		t.Fatal("did not expect uploader to be called")
	}
}

func TestAvatarHandler_Upload_FileTooLarge(t *testing.T) {
	uploader := &fakeAvatarUploader{
		err: domain.ErrFileTooLarge,
	}
	handler := NewAvatarHandler(uploader, services.DefaultMaxUploadSizeBytes)

	req := newMultipartUploadRequest(t, "avatar.jpg", "image/jpeg", []byte("data"))
	req.Header.Set("X-User-ID", "sergey")

	rec := httptest.NewRecorder()

	wrappedHandler := middleware.RequireUserID(http.HandlerFunc(handler.Upload))
	wrappedHandler.ServeHTTP(rec, req)

	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("unexpected status code: got %d, want %d", rec.Code, http.StatusRequestEntityTooLarge)
	}
}

func TestAvatarHandler_Upload_InvalidFile(t *testing.T) {
	uploader := &fakeAvatarUploader{
		err: domain.ErrInvalidFile,
	}
	handler := NewAvatarHandler(uploader, services.DefaultMaxUploadSizeBytes)

	req := newMultipartUploadRequest(t, "avatar.txt", "text/plain", []byte("data"))
	req.Header.Set("X-User-ID", "sergey")

	rec := httptest.NewRecorder()

	wrappedHandler := middleware.RequireUserID(http.HandlerFunc(handler.Upload))
	wrappedHandler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("unexpected status code: got %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestAvatarHandler_Upload_InternalError(t *testing.T) {
	uploader := &fakeAvatarUploader{
		err: errors.New("unexpected error"),
	}
	handler := NewAvatarHandler(uploader, services.DefaultMaxUploadSizeBytes)

	req := newMultipartUploadRequest(t, "avatar.jpg", "image/jpeg", []byte("data"))
	req.Header.Set("X-User-ID", "sergey")

	rec := httptest.NewRecorder()

	wrappedHandler := middleware.RequireUserID(http.HandlerFunc(handler.Upload))
	wrappedHandler.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("unexpected status code: got %d, want %d", rec.Code, http.StatusInternalServerError)
	}
}

func newMultipartUploadRequest(
	t *testing.T,
	fileName string,
	contentType string,
	data []byte,
) *http.Request {
	t.Helper()

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	partHeader := textproto.MIMEHeader{}
	partHeader.Set("Content-Disposition", `form-data; name="file"; filename="`+fileName+`"`)
	partHeader.Set("Content-Type", contentType)

	part, err := writer.CreatePart(partHeader)
	if err != nil {
		t.Fatalf("failed to create multipart part: %v", err)
	}

	if _, err := io.Copy(part, bytes.NewReader(data)); err != nil {
		t.Fatalf("failed to write multipart file: %v", err)
	}

	if err := writer.Close(); err != nil {
		t.Fatalf("failed to close multipart writer: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/avatars", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	return req
}

func TestAvatarHandler_GetByID_Success(t *testing.T) {
	uploader := &fakeAvatarUploader{
		getByIDResult: services.DownloadAvatarResult{
			Avatar: domain.Avatar{
				ID: "avatar-id",
			},
			Data:        []byte("image-data"),
			ContentType: "image/jpeg",
		},
	}

	handler := NewAvatarHandler(uploader, services.DefaultMaxUploadSizeBytes)

	router := chi.NewRouter()
	router.Get("/api/v1/avatars/{avatar_id}", handler.GetByID)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/avatars/avatar-id", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected status code: got %d, want %d", rec.Code, http.StatusOK)
	}

	if !uploader.getByIDCalled {
		t.Fatal("expected GetAvatarByID to be called")
	}

	if uploader.getByIDInput != "avatar-id" {
		t.Fatalf("unexpected avatar id: got %q, want %q", uploader.getByIDInput, "avatar-id")
	}

	if rec.Header().Get("Content-Type") != "image/jpeg" {
		t.Fatalf("unexpected content type: got %q", rec.Header().Get("Content-Type"))
	}

	if rec.Body.String() != "image-data" {
		t.Fatalf("unexpected body: got %q, want %q", rec.Body.String(), "image-data")
	}
}

func TestAvatarHandler_GetByID_NotFound(t *testing.T) {
	uploader := &fakeAvatarUploader{
		getByIDErr: domain.ErrAvatarNotFound,
	}

	handler := NewAvatarHandler(uploader, services.DefaultMaxUploadSizeBytes)

	router := chi.NewRouter()
	router.Get("/api/v1/avatars/{avatar_id}", handler.GetByID)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/avatars/avatar-id", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("unexpected status code: got %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestAvatarHandler_GetByID_InternalError(t *testing.T) {
	uploader := &fakeAvatarUploader{
		getByIDErr: errors.New("unexpected error"),
	}

	handler := NewAvatarHandler(uploader, services.DefaultMaxUploadSizeBytes)

	router := chi.NewRouter()
	router.Get("/api/v1/avatars/{avatar_id}", handler.GetByID)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/avatars/avatar-id", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("unexpected status code: got %d, want %d", rec.Code, http.StatusInternalServerError)
	}
}

func TestAvatarHandler_GetCurrentByUserID_Success(t *testing.T) {
	uploader := &fakeAvatarUploader{
		getCurrentResult: services.DownloadAvatarResult{
			Avatar: domain.Avatar{
				ID:     "avatar-id",
				UserID: "sergey",
			},
			Data:        []byte("image-data"),
			ContentType: "image/jpeg",
		},
	}

	handler := NewAvatarHandler(uploader, services.DefaultMaxUploadSizeBytes)

	router := chi.NewRouter()
	router.Get("/api/v1/users/{user_id}/avatar", handler.GetCurrentByUserID)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/users/sergey/avatar", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected status code: got %d, want %d", rec.Code, http.StatusOK)
	}

	if !uploader.getCurrentCalled {
		t.Fatal("expected GetCurrentAvatarByUserID to be called")
	}

	if uploader.getCurrentInput != "sergey" {
		t.Fatalf("unexpected user id: got %q, want %q", uploader.getCurrentInput, "sergey")
	}

	if rec.Header().Get("Content-Type") != "image/jpeg" {
		t.Fatalf("unexpected content type: got %q", rec.Header().Get("Content-Type"))
	}

	if rec.Body.String() != "image-data" {
		t.Fatalf("unexpected body: got %q, want %q", rec.Body.String(), "image-data")
	}
}

func TestAvatarHandler_GetCurrentByUserID_NotFound(t *testing.T) {
	uploader := &fakeAvatarUploader{
		getCurrentErr: domain.ErrAvatarNotFound,
	}

	handler := NewAvatarHandler(uploader, services.DefaultMaxUploadSizeBytes)

	router := chi.NewRouter()
	router.Get("/api/v1/users/{user_id}/avatar", handler.GetCurrentByUserID)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/users/unknown/avatar", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("unexpected status code: got %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestAvatarHandler_GetCurrentByUserID_InternalError(t *testing.T) {
	uploader := &fakeAvatarUploader{
		getCurrentErr: errors.New("unexpected error"),
	}

	handler := NewAvatarHandler(uploader, services.DefaultMaxUploadSizeBytes)

	router := chi.NewRouter()
	router.Get("/api/v1/users/{user_id}/avatar", handler.GetCurrentByUserID)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/users/sergey/avatar", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("unexpected status code: got %d, want %d", rec.Code, http.StatusInternalServerError)
	}
}

func TestAvatarHandler_GetMetadata_Success(t *testing.T) {
	createdAt := time.Date(2026, 5, 4, 12, 0, 0, 0, time.UTC)
	updatedAt := time.Date(2026, 5, 4, 12, 10, 0, 0, time.UTC)

	uploader := &fakeAvatarUploader{
		getMetadataAvatar: domain.Avatar{
			ID:        "avatar-id",
			UserID:    "sergey",
			FileName:  "avatar.jpg",
			MIMEType:  "image/jpeg",
			SizeBytes: 1024,
			Width:     800,
			Height:    600,
			ThumbnailS3Keys: map[domain.ThumbnailSize]string{
				domain.ThumbnailSize100: "thumbnails/avatar-id/100x100.jpg",
				domain.ThumbnailSize300: "thumbnails/avatar-id/300x300.jpg",
			},
			CreatedAt: createdAt,
			UpdatedAt: updatedAt,
		},
	}

	handler := NewAvatarHandler(uploader, services.DefaultMaxUploadSizeBytes)

	router := chi.NewRouter()
	router.Get("/api/v1/avatars/{avatar_id}/metadata", handler.GetMetadata)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/avatars/avatar-id/metadata", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected status code: got %d, want %d", rec.Code, http.StatusOK)
	}

	if !uploader.getMetadataCalled {
		t.Fatal("expected GetAvatarMetadata to be called")
	}

	if uploader.getMetadataInput != "avatar-id" {
		t.Fatalf("unexpected avatar id: got %q, want %q", uploader.getMetadataInput, "avatar-id")
	}

	var response AvatarMetadataResponse
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if response.ID != "avatar-id" {
		t.Fatalf("unexpected id: got %q, want %q", response.ID, "avatar-id")
	}

	if response.UserID != "sergey" {
		t.Fatalf("unexpected user id: got %q, want %q", response.UserID, "sergey")
	}

	if response.FileName != "avatar.jpg" {
		t.Fatalf("unexpected file name: got %q, want %q", response.FileName, "avatar.jpg")
	}

	if response.MIMEType != "image/jpeg" {
		t.Fatalf("unexpected mime type: got %q, want %q", response.MIMEType, "image/jpeg")
	}

	if response.Size != 1024 {
		t.Fatalf("unexpected size: got %d, want %d", response.Size, 1024)
	}

	if response.Dimensions.Width != 800 {
		t.Fatalf("unexpected width: got %d, want %d", response.Dimensions.Width, 800)
	}

	if response.Dimensions.Height != 600 {
		t.Fatalf("unexpected height: got %d, want %d", response.Dimensions.Height, 600)
	}

	if len(response.Thumbnails) != 2 {
		t.Fatalf("unexpected thumbnails count: got %d, want %d", len(response.Thumbnails), 2)
	}
}

func TestAvatarHandler_GetMetadata_NotFound(t *testing.T) {
	uploader := &fakeAvatarUploader{
		getMetadataErr: domain.ErrAvatarNotFound,
	}

	handler := NewAvatarHandler(uploader, services.DefaultMaxUploadSizeBytes)

	router := chi.NewRouter()
	router.Get("/api/v1/avatars/{avatar_id}/metadata", handler.GetMetadata)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/avatars/avatar-id/metadata", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("unexpected status code: got %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestAvatarHandler_GetMetadata_InternalError(t *testing.T) {
	uploader := &fakeAvatarUploader{
		getMetadataErr: errors.New("unexpected error"),
	}

	handler := NewAvatarHandler(uploader, services.DefaultMaxUploadSizeBytes)

	router := chi.NewRouter()
	router.Get("/api/v1/avatars/{avatar_id}/metadata", handler.GetMetadata)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/avatars/avatar-id/metadata", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("unexpected status code: got %d, want %d", rec.Code, http.StatusInternalServerError)
	}
}

func TestAvatarHandler_ListByUserID_Success(t *testing.T) {
	createdAt := time.Date(2026, 5, 4, 12, 0, 0, 0, time.UTC)

	uploader := &fakeAvatarUploader{
		listByUserIDAvatars: []domain.Avatar{
			{
				ID:        "avatar-1",
				UserID:    "sergey",
				FileName:  "avatar-1.jpg",
				MIMEType:  "image/jpeg",
				SizeBytes: 1024,
				Width:     800,
				Height:    600,
				CreatedAt: createdAt,
				UpdatedAt: createdAt,
			},
			{
				ID:        "avatar-2",
				UserID:    "sergey",
				FileName:  "avatar-2.jpg",
				MIMEType:  "image/jpeg",
				SizeBytes: 2048,
				Width:     1024,
				Height:    768,
				CreatedAt: createdAt,
				UpdatedAt: createdAt,
			},
		},
	}

	handler := NewAvatarHandler(uploader, services.DefaultMaxUploadSizeBytes)

	router := chi.NewRouter()
	router.Get("/api/v1/users/{user_id}/avatars", handler.ListByUserID)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/users/sergey/avatars", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected status code: got %d, want %d", rec.Code, http.StatusOK)
	}

	if !uploader.listByUserIDCalled {
		t.Fatal("expected ListAvatarsByUserID to be called")
	}

	if uploader.listByUserIDInput != "sergey" {
		t.Fatalf("unexpected user id: got %q, want %q", uploader.listByUserIDInput, "sergey")
	}

	var response UserAvatarsResponse
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if response.UserID != "sergey" {
		t.Fatalf("unexpected user id: got %q, want %q", response.UserID, "sergey")
	}

	if len(response.Avatars) != 2 {
		t.Fatalf("unexpected avatars count: got %d, want %d", len(response.Avatars), 2)
	}

	if response.Avatars[0].ID != "avatar-1" {
		t.Fatalf("unexpected first avatar id: got %q, want %q", response.Avatars[0].ID, "avatar-1")
	}
}

func TestAvatarHandler_ListByUserID_EmptyList(t *testing.T) {
	uploader := &fakeAvatarUploader{
		listByUserIDAvatars: []domain.Avatar{},
	}

	handler := NewAvatarHandler(uploader, services.DefaultMaxUploadSizeBytes)

	router := chi.NewRouter()
	router.Get("/api/v1/users/{user_id}/avatars", handler.ListByUserID)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/users/sergey/avatars", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected status code: got %d, want %d", rec.Code, http.StatusOK)
	}

	var response UserAvatarsResponse
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if response.UserID != "sergey" {
		t.Fatalf("unexpected user id: got %q, want %q", response.UserID, "sergey")
	}

	if len(response.Avatars) != 0 {
		t.Fatalf("unexpected avatars count: got %d, want %d", len(response.Avatars), 0)
	}
}

func TestAvatarHandler_ListByUserID_InternalError(t *testing.T) {
	uploader := &fakeAvatarUploader{
		listByUserIDErr: errors.New("unexpected error"),
	}

	handler := NewAvatarHandler(uploader, services.DefaultMaxUploadSizeBytes)

	router := chi.NewRouter()
	router.Get("/api/v1/users/{user_id}/avatars", handler.ListByUserID)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/users/sergey/avatars", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("unexpected status code: got %d, want %d", rec.Code, http.StatusInternalServerError)
	}
}

func TestAvatarHandler_DeleteByID_Success(t *testing.T) {
	uploader := &fakeAvatarUploader{
		deleteAvatar: domain.Avatar{
			ID:     "avatar-id",
			UserID: "sergey",
		},
	}

	handler := NewAvatarHandler(uploader, services.DefaultMaxUploadSizeBytes)

	router := chi.NewRouter()
	router.With(middleware.RequireUserID).Delete("/api/v1/avatars/{avatar_id}", handler.DeleteByID)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/avatars/avatar-id", nil)
	req.Header.Set("X-User-ID", "sergey")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("unexpected status code: got %d, want %d", rec.Code, http.StatusNoContent)
	}

	if !uploader.deleteCalled {
		t.Fatal("expected DeleteAvatarByID to be called")
	}

	if uploader.deleteAvatarID != "avatar-id" {
		t.Fatalf("unexpected avatar id: got %q, want %q", uploader.deleteAvatarID, "avatar-id")
	}

	if uploader.deleteUserID != "sergey" {
		t.Fatalf("unexpected user id: got %q, want %q", uploader.deleteUserID, "sergey")
	}
}

func TestAvatarHandler_DeleteByID_MissingUserID(t *testing.T) {
	uploader := &fakeAvatarUploader{}
	handler := NewAvatarHandler(uploader, services.DefaultMaxUploadSizeBytes)

	router := chi.NewRouter()
	router.With(middleware.RequireUserID).Delete("/api/v1/avatars/{avatar_id}", handler.DeleteByID)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/avatars/avatar-id", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("unexpected status code: got %d, want %d", rec.Code, http.StatusBadRequest)
	}

	if uploader.deleteCalled {
		t.Fatal("did not expect DeleteAvatarByID to be called")
	}
}

func TestAvatarHandler_DeleteByID_Forbidden(t *testing.T) {
	uploader := &fakeAvatarUploader{
		deleteErr: domain.ErrForbidden,
	}

	handler := NewAvatarHandler(uploader, services.DefaultMaxUploadSizeBytes)

	router := chi.NewRouter()
	router.With(middleware.RequireUserID).Delete("/api/v1/avatars/{avatar_id}", handler.DeleteByID)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/avatars/avatar-id", nil)
	req.Header.Set("X-User-ID", "ivan")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("unexpected status code: got %d, want %d", rec.Code, http.StatusForbidden)
	}
}

func TestAvatarHandler_DeleteByID_NotFound(t *testing.T) {
	uploader := &fakeAvatarUploader{
		deleteErr: domain.ErrAvatarNotFound,
	}

	handler := NewAvatarHandler(uploader, services.DefaultMaxUploadSizeBytes)

	router := chi.NewRouter()
	router.With(middleware.RequireUserID).Delete("/api/v1/avatars/{avatar_id}", handler.DeleteByID)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/avatars/avatar-id", nil)
	req.Header.Set("X-User-ID", "sergey")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("unexpected status code: got %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestAvatarHandler_DeleteByID_InternalError(t *testing.T) {
	uploader := &fakeAvatarUploader{
		deleteErr: errors.New("unexpected error"),
	}

	handler := NewAvatarHandler(uploader, services.DefaultMaxUploadSizeBytes)

	router := chi.NewRouter()
	router.With(middleware.RequireUserID).Delete("/api/v1/avatars/{avatar_id}", handler.DeleteByID)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/avatars/avatar-id", nil)
	req.Header.Set("X-User-ID", "sergey")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("unexpected status code: got %d, want %d", rec.Code, http.StatusInternalServerError)
	}
}
