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
)

type fakeAvatarUploader struct {
	called bool
	input  services.UploadAvatarInput
	err    error

	avatar domain.Avatar
}

func (u *fakeAvatarUploader) UploadAvatar(ctx context.Context, input services.UploadAvatarInput) (domain.Avatar, error) {
	u.called = true
	u.input = input

	if u.err != nil {
		return domain.Avatar{}, u.err
	}

	return u.avatar, nil
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
