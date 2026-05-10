package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"mime/multipart"
	"net/http"
	"sort"
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
	GetAvatarThumbnailByID(ctx context.Context, avatarID string, size domain.ThumbnailSize) (services.DownloadAvatarResult, error)
	GetCurrentAvatarByUserID(ctx context.Context, userID string) (services.DownloadAvatarResult, error)
	GetAvatarMetadata(ctx context.Context, avatarID string) (domain.Avatar, error)
	ListAvatarsByUserID(ctx context.Context, userID string) ([]domain.Avatar, error)
	DeleteAvatarByID(ctx context.Context, avatarID string, userID string) (domain.Avatar, error)
	DeleteCurrentAvatarByUserID(ctx context.Context, userID string, actorUserID string) (domain.Avatar, error)
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

// AvatarMetadataResponse описывает ответ с метаданными аватарки.
type AvatarMetadataResponse struct {
	ID         string                    `json:"id"`
	UserID     string                    `json:"user_id"`
	FileName   string                    `json:"file_name"`
	MIMEType   string                    `json:"mime_type"`
	Size       int64                     `json:"size"`
	Dimensions AvatarDimensionsResponse  `json:"dimensions"`
	Thumbnails []AvatarThumbnailResponse `json:"thumbnails"`
	CreatedAt  time.Time                 `json:"created_at"`
	UpdatedAt  time.Time                 `json:"updated_at"`
}

// UserAvatarsResponse описывает список аватарок пользователя.
type UserAvatarsResponse struct {
	UserID  string                   `json:"user_id"`
	Avatars []AvatarMetadataResponse `json:"avatars"`
}

// AvatarDimensionsResponse описывает размеры изображения.
type AvatarDimensionsResponse struct {
	Width  int `json:"width"`
	Height int `json:"height"`
}

// AvatarThumbnailResponse описывает миниатюру изображения.
type AvatarThumbnailResponse struct {
	Size string `json:"size"`
	URL  string `json:"url"`
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

	file, fileHeader, err := formFileByNames(r, "file", "image")
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

	sizeParam := strings.TrimSpace(r.URL.Query().Get("size"))
	if sizeParam != "" {
		result, err := h.avatarService.GetAvatarThumbnailByID(
			r.Context(),
			avatarID,
			domain.ThumbnailSize(sizeParam),
		)
		if err != nil {
			h.handleGetByIDError(w, err)
			return
		}

		writeAvatarBinary(w, result)
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

// GetMetadata обрабатывает получение метаданных аватарки.
func (h *AvatarHandler) GetMetadata(w http.ResponseWriter, r *http.Request) {
	avatarID := strings.TrimSpace(chi.URLParam(r, "avatar_id"))
	if avatarID == "" {
		writeJSONError(w, http.StatusBadRequest, ErrorResponse{
			Error:   "Invalid avatar id",
			Details: "avatar_id is required",
		})
		return
	}

	avatar, err := h.avatarService.GetAvatarMetadata(r.Context(), avatarID)
	if err != nil {
		h.handleGetByIDError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, buildAvatarMetadataResponse(avatar))
}

// ListByUserID обрабатывает получение списка аватарок пользователя.
func (h *AvatarHandler) ListByUserID(w http.ResponseWriter, r *http.Request) {
	userID := strings.TrimSpace(chi.URLParam(r, "user_id"))
	if userID == "" {
		writeJSONError(w, http.StatusBadRequest, ErrorResponse{
			Error:   "Invalid user id",
			Details: "user_id is required",
		})
		return
	}

	avatars, err := h.avatarService.ListAvatarsByUserID(r.Context(), userID)
	if err != nil {
		h.handleListByUserIDError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, UserAvatarsResponse{
		UserID:  userID,
		Avatars: buildAvatarMetadataResponses(avatars),
	})
}

// DeleteByID обрабатывает мягкое удаление аватарки по id.
func (h *AvatarHandler) DeleteByID(w http.ResponseWriter, r *http.Request) {
	avatarID := strings.TrimSpace(chi.URLParam(r, "avatar_id"))
	if avatarID == "" {
		writeJSONError(w, http.StatusBadRequest, ErrorResponse{
			Error:   "Invalid avatar id",
			Details: "avatar_id is required",
		})
		return
	}

	userID, ok := middleware.UserIDFromContext(r.Context())
	if !ok {
		writeJSONError(w, http.StatusBadRequest, ErrorResponse{
			Error:   "Missing user id",
			Details: "Required header: X-User-ID",
		})
		return
	}

	_, err := h.avatarService.DeleteAvatarByID(r.Context(), avatarID, userID)
	if err != nil {
		h.handleDeleteByIDError(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// DeleteCurrentByUserID обрабатывает мягкое удаление текущей аватарки пользователя.
func (h *AvatarHandler) DeleteCurrentByUserID(w http.ResponseWriter, r *http.Request) {
	userID := strings.TrimSpace(chi.URLParam(r, "user_id"))
	if userID == "" {
		writeJSONError(w, http.StatusBadRequest, ErrorResponse{
			Error:   "Invalid user id",
			Details: "user_id is required",
		})
		return
	}

	actorUserID, ok := middleware.UserIDFromContext(r.Context())
	if !ok {
		writeJSONError(w, http.StatusBadRequest, ErrorResponse{
			Error:   "Missing user id",
			Details: "Required header: X-User-ID",
		})
		return
	}

	_, err := h.avatarService.DeleteCurrentAvatarByUserID(r.Context(), userID, actorUserID)
	if err != nil {
		h.handleDeleteByIDError(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// handleDeleteByIDError преобразует ошибки удаления аватарки в HTTP-ответы.
func (h *AvatarHandler) handleDeleteByIDError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, domain.ErrAvatarNotFound), errors.Is(err, domain.ErrAvatarDeleted):
		writeJSONError(w, http.StatusNotFound, ErrorResponse{
			Error: "Avatar not found",
		})
	case errors.Is(err, domain.ErrForbidden):
		writeJSONError(w, http.StatusForbidden, ErrorResponse{
			Error:   "Forbidden",
			Details: "You can only delete your own avatars",
		})
	case errors.Is(err, domain.ErrMissingUserID):
		writeJSONError(w, http.StatusBadRequest, ErrorResponse{
			Error:   "Missing user id",
			Details: "Required header: X-User-ID",
		})
	default:
		writeJSONError(w, http.StatusInternalServerError, ErrorResponse{
			Error: "Internal server error",
		})
	}
}

// handleListByUserIDError преобразует ошибки получения списка аватарок в HTTP-ответы.
func (h *AvatarHandler) handleListByUserIDError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, domain.ErrMissingUserID):
		writeJSONError(w, http.StatusBadRequest, ErrorResponse{
			Error:   "Invalid user id",
			Details: "user_id is required",
		})
	default:
		writeJSONError(w, http.StatusInternalServerError, ErrorResponse{
			Error: "Internal server error",
		})
	}
}

// buildAvatarMetadataResponses преобразует список domain.Avatar в список HTTP response metadata.
func buildAvatarMetadataResponses(avatars []domain.Avatar) []AvatarMetadataResponse {
	if len(avatars) == 0 {
		return []AvatarMetadataResponse{}
	}

	responses := make([]AvatarMetadataResponse, 0, len(avatars))
	for _, avatar := range avatars {
		responses = append(responses, buildAvatarMetadataResponse(avatar))
	}

	return responses
}

// handleGetByIDError преобразует ошибки получения аватарки в HTTP-ответы.
func (h *AvatarHandler) handleGetByIDError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, domain.ErrAvatarNotFound), errors.Is(err, domain.ErrAvatarDeleted):
		writeJSONError(w, http.StatusNotFound, ErrorResponse{
			Error: "Avatar not found",
		})
	case errors.Is(err, domain.ErrThumbnailNotFound):
		writeJSONError(w, http.StatusNotFound, ErrorResponse{
			Error: "Thumbnail not found",
		})
	case errors.Is(err, domain.ErrInvalidThumbnailSize):
		writeJSONError(w, http.StatusBadRequest, ErrorResponse{
			Error:   "Invalid thumbnail size",
			Details: "Supported sizes: 100x100, 300x300",
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

// buildAvatarMetadataResponse преобразует domain.Avatar в HTTP response metadata.
func buildAvatarMetadataResponse(avatar domain.Avatar) AvatarMetadataResponse {
	return AvatarMetadataResponse{
		ID:       avatar.ID,
		UserID:   avatar.UserID,
		FileName: avatar.FileName,
		MIMEType: avatar.MIMEType,
		Size:     avatar.SizeBytes,
		Dimensions: AvatarDimensionsResponse{
			Width:  avatar.Width,
			Height: avatar.Height,
		},
		Thumbnails: buildAvatarThumbnailResponses(avatar),
		CreatedAt:  avatar.CreatedAt,
		UpdatedAt:  avatar.UpdatedAt,
	}
}

// buildAvatarThumbnailResponses формирует список thumbnails для ответа metadata.
func buildAvatarThumbnailResponses(avatar domain.Avatar) []AvatarThumbnailResponse {
	if len(avatar.ThumbnailS3Keys) == 0 {
		return []AvatarThumbnailResponse{}
	}

	sizes := make([]string, 0, len(avatar.ThumbnailS3Keys))
	for size, key := range avatar.ThumbnailS3Keys {
		if strings.TrimSpace(key) == "" {
			continue
		}

		sizes = append(sizes, string(size))
	}

	sort.Strings(sizes)

	thumbnails := make([]AvatarThumbnailResponse, 0, len(sizes))
	for _, size := range sizes {
		thumbnails = append(thumbnails, AvatarThumbnailResponse{
			Size: size,
			URL:  fmt.Sprintf("/api/v1/avatars/%s?size=%s", avatar.ID, size),
		})
	}

	return thumbnails
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

// formFileByNames ищет файл в multipart/form-data по нескольким возможным именам поля.
func formFileByNames(
	r *http.Request,
	fieldNames ...string,
) (multipart.File, *multipart.FileHeader, error) {
	var lastErr error

	for _, fieldName := range fieldNames {
		file, fileHeader, err := r.FormFile(fieldName)
		if err == nil {
			return file, fileHeader, nil
		}

		lastErr = err
	}

	return nil, nil, lastErr
}
