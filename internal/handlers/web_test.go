package handlers

import (
	"bytes"
	"context"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"strings"
	"testing"

	"github.com/Dyuzhovsergey/gophprofile/internal/domain"
	"github.com/Dyuzhovsergey/gophprofile/internal/services"
	"github.com/go-chi/chi/v5"
)

type fakeWebAvatarManager struct {
	uploadCalled bool
	uploadInput  services.UploadAvatarInput
	uploadAvatar domain.Avatar
	uploadErr    error

	listCalled  bool
	listUserID  string
	listAvatars []domain.Avatar
	listErr     error
}

func (m *fakeWebAvatarManager) UploadAvatar(
	ctx context.Context,
	input services.UploadAvatarInput,
) (domain.Avatar, error) {
	m.uploadCalled = true
	m.uploadInput = input

	if m.uploadErr != nil {
		return domain.Avatar{}, m.uploadErr
	}

	return m.uploadAvatar, nil
}

func (m *fakeWebAvatarManager) ListAvatarsByUserID(
	ctx context.Context,
	userID string,
) ([]domain.Avatar, error) {
	m.listCalled = true
	m.listUserID = userID

	if m.listErr != nil {
		return nil, m.listErr
	}

	return m.listAvatars, nil
}

func TestWebHandler_Upload_SuccessWithImageField(t *testing.T) {
	manager := &fakeWebAvatarManager{
		uploadAvatar: domain.Avatar{
			ID:     "avatar-id",
			UserID: "sergey",
		},
	}

	handler := NewWebHandler(manager, services.DefaultMaxUploadSizeBytes)

	req := newWebUploadRequest(t, "user_id", "sergey", "image", "avatar.png", "image/png", []byte("data"))
	rec := httptest.NewRecorder()

	handler.Upload(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("unexpected status code: got %d, want %d", rec.Code, http.StatusSeeOther)
	}

	if !manager.uploadCalled {
		t.Fatal("expected UploadAvatar to be called")
	}

	if manager.uploadInput.UserID != "sergey" {
		t.Fatalf("unexpected user id: got %q, want %q", manager.uploadInput.UserID, "sergey")
	}

	if manager.uploadInput.FileName != "avatar.png" {
		t.Fatalf("unexpected file name: got %q, want %q", manager.uploadInput.FileName, "avatar.png")
	}

	location := rec.Header().Get("Location")
	if location != "/web/gallery/sergey?uploaded=avatar-id" {
		t.Fatalf("unexpected redirect location: got %q", location)
	}
}

func TestWebHandler_Gallery(t *testing.T) {
	manager := &fakeWebAvatarManager{
		listAvatars: []domain.Avatar{
			{
				ID:        "avatar-id",
				UserID:    "sergey",
				FileName:  "avatar.png",
				MIMEType:  "image/png",
				SizeBytes: 10,
			},
		},
	}

	handler := NewWebHandler(manager, services.DefaultMaxUploadSizeBytes)

	router := chi.NewRouter()
	router.Get("/web/gallery/{user_id}", handler.Gallery)

	req := httptest.NewRequest(http.MethodGet, "/web/gallery/sergey", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected status code: got %d, want %d", rec.Code, http.StatusOK)
	}

	if !manager.listCalled {
		t.Fatal("expected ListAvatarsByUserID to be called")
	}

	if manager.listUserID != "sergey" {
		t.Fatalf("unexpected user id: got %q, want %q", manager.listUserID, "sergey")
	}

	if !strings.Contains(rec.Body.String(), "avatar-id") {
		t.Fatal("expected avatar id in gallery response")
	}
}

func newWebUploadRequest(
	t *testing.T,
	userFieldName string,
	userID string,
	fileFieldName string,
	fileName string,
	contentType string,
	data []byte,
) *http.Request {
	t.Helper()

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	if err := writer.WriteField(userFieldName, userID); err != nil {
		t.Fatalf("failed to write user field: %v", err)
	}

	partHeader := textproto.MIMEHeader{}
	partHeader.Set("Content-Disposition", `form-data; name="`+fileFieldName+`"; filename="`+fileName+`"`)
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

	req := httptest.NewRequest(http.MethodPost, "/web/upload", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	return req
}
