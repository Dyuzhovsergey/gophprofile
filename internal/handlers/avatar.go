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
	"github.com/go-chi/chi/v5"
)

// AvatarManager описывает сервис управления аватарками.
type AvatarManager interface {
	UploadAvatar(ctx context.Context, input services.UploadAvatarInput) (domain.Avatar, error)
	GetAvatarByID(ctx context.Context, avatarID string) (services.DownloadAvatarResult, error)
	GetCurrentAvatarByUserID(ctx context.Context, userID string) (services.DownloadAvatarResult, error)
}

// AvatarHandler содержит HTTP-обработчики для работы с аватарками.
type AvatarHandler struct {
	avatarService      AvatarManager
	maxUploadSizeBytes int64
}

// NewAvatarHandler создаёт обработчик аватарок.
func NewAvatarHandler(avatarService AvatarManager, maxUploadSizeBytes int64) *AvatarHandler {
	if maxUploadSizeBytes <= 0 {
		maxUploadSizeBytes = services.DefaultMaxUploadSizeBytes
	}

	return &AvatarHandler{
		avatarService:      avatarService,
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

	avatar, err := h.avatarService.UploadAvatar(r.Context(), services.UploadAvatarInput{
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

// GetByID обрабатывает получение аватарки по id.
func (h *AvatarHandler) GetByID(w http.ResponseWriter, r *http.Request) {
	avatarID := strings.TrimSpace(chi.URLParam(r, "avatar_id"))
	if avatarID == "" {
		writeJSONError(w, http.StatusBadRequest, ErrorResponse{
			Error:   "Invalid avatar id",
			Details: "avatar_id is required",
		})
		return
	}

	result, err := h.avatarService.GetAvatarByID(r.Context(), avatarID)
	if err != nil {
		h.handleGetByIDError(w, err)
		return
	}

	writeAvatarBinary(w, result)
}

// GetCurrentByUserID обрабатывает получение текущей аватарки пользователя.
func (h *AvatarHandler) GetCurrentByUserID(w http.ResponseWriter, r *http.Request) {
	userID := strings.TrimSpace(chi.URLParam(r, "user_id"))
	if userID == "" {
		writeJSONError(w, http.StatusBadRequest, ErrorResponse{
			Error:   "Invalid user id",
			Details: "user_id is required",
		})
		return
	}

	result, err := h.avatarService.GetCurrentAvatarByUserID(r.Context(), userID)
	if err != nil {
		h.handleGetByIDError(w, err)
		return
	}

	writeAvatarBinary(w, result)
}

// handleGetByIDError преобразует ошибки получения аватарки в HTTP-ответы.
func (h *AvatarHandler) handleGetByIDError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, domain.ErrAvatarNotFound), errors.Is(err, domain.ErrAvatarDeleted):
		writeJSONError(w, http.StatusNotFound, ErrorResponse{
			Error: "Avatar not found",
		})
	default:
		writeJSONError(w, http.StatusInternalServerError, ErrorResponse{
			Error: "Internal server error",
		})
	}
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

// writeAvatarBinary записывает бинарные данные аватарки в HTTP-ответ.
func writeAvatarBinary(w http.ResponseWriter, result services.DownloadAvatarResult) {
	contentType := strings.TrimSpace(result.ContentType)
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Length", fmt.Sprintf("%d", len(result.Data)))
	w.WriteHeader(http.StatusOK)

	if _, err := w.Write(result.Data); err != nil {
		return
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
