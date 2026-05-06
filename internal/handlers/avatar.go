package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/Dyuzhovsergey/gophprofile/internal/domain"
	"github.com/Dyuzhovsergey/gophprofile/internal/middleware"
	"github.com/Dyuzhovsergey/gophprofile/internal/services"
)

// AvatarUploader описывает сервис загрузки аватарок.
type AvatarUploader interface {
	UploadAvatar(ctx context.Context, input services.UploadAvatarInput) (domain.Avatar, error)
}

// AvatarHandler содержит HTTP-обработчики для работы с аватарками.
type AvatarHandler struct {
	uploader           AvatarUploader
	maxUploadSizeBytes int64
}

// NewAvatarHandler создаёт обработчик аватарок.
func NewAvatarHandler(uploader AvatarUploader, maxUploadSizeBytes int64) *AvatarHandler {
	if maxUploadSizeBytes <= 0 {
		maxUploadSizeBytes = services.DefaultMaxUploadSizeBytes
	}

	return &AvatarHandler{
		uploader:           uploader,
		maxUploadSizeBytes: maxUploadSizeBytes,
	}
}

// UploadAvatarResponse описывает ответ успешной загрузки аватарки.
type UploadAvatarResponse struct {
	ID        string    `json:"id"`
	UserID    string    `json:"user_id"`
	URL       string    `json:"url"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
}

// ErrorResponse описывает JSON-ответ с ошибкой.
type ErrorResponse struct {
	Error   string `json:"error"`
	Details string `json:"details,omitempty"`
	MaxSize int64  `json:"max_size,omitempty"`
}

// Upload обрабатывает загрузку аватарки.
func (h *AvatarHandler) Upload(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.UserIDFromContext(r.Context())
	if !ok {
		writeJSONError(w, http.StatusBadRequest, ErrorResponse{
			Error:   "Missing user id",
			Details: "Required header: X-User-ID",
		})
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, h.maxUploadSizeBytes)

	file, fileHeader, err := r.FormFile("file")
	if err != nil {
		h.handleMultipartError(w, err)
		return
	}
	defer file.Close()

	avatar, err := h.uploader.UploadAvatar(r.Context(), services.UploadAvatarInput{
		UserID:    userID,
		FileName:  fileHeader.Filename,
		MIMEType:  fileHeader.Header.Get("Content-Type"),
		SizeBytes: fileHeader.Size,
		Body:      file,
	})
	if err != nil {
		h.handleUploadError(w, err)
		return
	}

	writeJSON(w, http.StatusCreated, UploadAvatarResponse{
		ID:        avatar.ID,
		UserID:    avatar.UserID,
		URL:       fmt.Sprintf("/api/v1/avatars/%s", avatar.ID),
		Status:    string(avatar.ProcessingStatus),
		CreatedAt: avatar.CreatedAt,
	})
}

// handleMultipartError обрабатывает ошибки чтения multipart/form-data.
func (h *AvatarHandler) handleMultipartError(w http.ResponseWriter, err error) {
	var maxBytesErr *http.MaxBytesError
	if errors.As(err, &maxBytesErr) || strings.Contains(err.Error(), "request body too large") {
		writeJSONError(w, http.StatusRequestEntityTooLarge, ErrorResponse{
			Error:   "File too large",
			MaxSize: h.maxUploadSizeBytes,
		})
		return
	}

	writeJSONError(w, http.StatusBadRequest, ErrorResponse{
		Error:   "Invalid multipart form",
		Details: "Required form field: file",
	})
}

// handleUploadError преобразует доменные ошибки загрузки в HTTP-ответы.
func (h *AvatarHandler) handleUploadError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, domain.ErrMissingUserID):
		writeJSONError(w, http.StatusBadRequest, ErrorResponse{
			Error:   "Missing user id",
			Details: "Required header: X-User-ID",
		})
	case errors.Is(err, domain.ErrFileTooLarge):
		writeJSONError(w, http.StatusRequestEntityTooLarge, ErrorResponse{
			Error:   "File too large",
			MaxSize: h.maxUploadSizeBytes,
		})
	case errors.Is(err, domain.ErrInvalidFile):
		writeJSONError(w, http.StatusBadRequest, ErrorResponse{
			Error:   "Invalid file format",
			Details: "Supported formats: jpeg, png, webp",
		})
	default:
		writeJSONError(w, http.StatusInternalServerError, ErrorResponse{
			Error: "Internal server error",
		})
	}
}

// writeJSON записывает JSON-ответ.
func writeJSON(w http.ResponseWriter, statusCode int, response any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, "failed to encode response", http.StatusInternalServerError)
	}
}

// writeJSONError записывает JSON-ответ с ошибкой.
func writeJSONError(w http.ResponseWriter, statusCode int, response ErrorResponse) {
	writeJSON(w, statusCode, response)
}
