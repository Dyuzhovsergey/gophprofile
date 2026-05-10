package handlers

import (
	"context"
	"errors"
	"fmt"
	"html/template"
	"net/http"
	"net/url"
	"strings"

	"github.com/Dyuzhovsergey/gophprofile/internal/domain"
	"github.com/Dyuzhovsergey/gophprofile/internal/services"
	"github.com/go-chi/chi/v5"
)

// WebAvatarManager описывает методы сервиса, которые нужны веб-интерфейсу.
type WebAvatarManager interface {
	UploadAvatar(ctx context.Context, input services.UploadAvatarInput) (domain.Avatar, error)
	ListAvatarsByUserID(ctx context.Context, userID string) ([]domain.Avatar, error)
}

// WebHandler содержит обработчики HTML-страниц.
type WebHandler struct {
	avatarService      WebAvatarManager
	maxUploadSizeBytes int64
	templates          *template.Template
}

// NewWebHandler создаёт обработчик веб-интерфейса.
func NewWebHandler(avatarService WebAvatarManager, maxUploadSizeBytes int64) (*WebHandler, error) {
	templates, err := template.ParseFiles(
		"web/templates/upload.html",
		"web/templates/gallery.html",
	)
	if err != nil {
		return nil, fmt.Errorf("parse web templates: %w", err)
	}

	return newWebHandlerWithTemplates(avatarService, maxUploadSizeBytes, templates), nil
}

// newWebHandlerWithTemplates создаёт WebHandler с уже подготовленными шаблонами.
func newWebHandlerWithTemplates(
	avatarService WebAvatarManager,
	maxUploadSizeBytes int64,
	templates *template.Template,
) *WebHandler {
	if maxUploadSizeBytes <= 0 {
		maxUploadSizeBytes = services.DefaultMaxUploadSizeBytes
	}

	return &WebHandler{
		avatarService:      avatarService,
		maxUploadSizeBytes: maxUploadSizeBytes,
		templates:          templates,
	}
}

// UploadPage отображает HTML-форму загрузки аватарки.
func (h *WebHandler) UploadPage(w http.ResponseWriter, r *http.Request) {
	h.renderUploadPage(w, uploadPageData{})
}

// Upload обрабатывает загрузку аватарки из HTML-формы.
func (h *WebHandler) Upload(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, h.maxUploadSizeBytes)

	if err := r.ParseMultipartForm(h.maxUploadSizeBytes); err != nil {
		h.renderUploadPage(w, uploadPageData{
			Error: "Не удалось прочитать форму. Проверьте размер файла.",
		})
		return
	}

	userID := strings.TrimSpace(r.FormValue("user_id"))
	if userID == "" {
		h.renderUploadPage(w, uploadPageData{
			Error: "Укажите User ID.",
		})
		return
	}

	file, fileHeader, err := r.FormFile("file")
	if err != nil {
		h.renderUploadPage(w, uploadPageData{
			UserID: userID,
			Error:  "Выберите файл аватарки.",
		})
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
		h.renderUploadPage(w, uploadPageData{
			UserID: userID,
			Error:  webUploadErrorMessage(err),
		})
		return
	}

	redirectURL := fmt.Sprintf(
		"/web/gallery/%s?uploaded=%s",
		url.PathEscape(userID),
		url.QueryEscape(avatar.ID),
	)

	http.Redirect(w, r, redirectURL, http.StatusSeeOther)
}

// Gallery отображает галерею активных аватарок пользователя.
func (h *WebHandler) Gallery(w http.ResponseWriter, r *http.Request) {
	userID := strings.TrimSpace(chi.URLParam(r, "user_id"))
	if userID == "" {
		http.Error(w, "user_id is required", http.StatusBadRequest)
		return
	}

	avatars, err := h.avatarService.ListAvatarsByUserID(r.Context(), userID)
	if err != nil {
		http.Error(w, "failed to load gallery", http.StatusInternalServerError)
		return
	}

	h.renderGalleryPage(w, galleryPageData{
		UserID:     userID,
		UploadedID: strings.TrimSpace(r.URL.Query().Get("uploaded")),
		Avatars:    buildAvatarMetadataResponses(avatars),
	})
}

// uploadPageData содержит данные страницы загрузки.
type uploadPageData struct {
	UserID string
	Error  string
}

// galleryPageData содержит данные страницы галереи.
type galleryPageData struct {
	UserID     string
	UploadedID string
	Avatars    []AvatarMetadataResponse
}

// renderUploadPage рендерит страницу загрузки.
func (h *WebHandler) renderUploadPage(w http.ResponseWriter, data uploadPageData) {
	h.renderTemplate(w, "upload.html", data)
}

// renderGalleryPage рендерит страницу галереи.
func (h *WebHandler) renderGalleryPage(w http.ResponseWriter, data galleryPageData) {
	h.renderTemplate(w, "gallery.html", data)
}

// renderTemplate выполняет HTML-шаблон.
func (h *WebHandler) renderTemplate(w http.ResponseWriter, name string, data any) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	if err := h.templates.ExecuteTemplate(w, name, data); err != nil {
		http.Error(w, "failed to render page", http.StatusInternalServerError)
		return
	}
}

// webUploadErrorMessage возвращает пользовательское сообщение об ошибке загрузки.
func webUploadErrorMessage(err error) string {
	switch {
	case errors.Is(err, domain.ErrMissingUserID):
		return "Укажите User ID."
	case errors.Is(err, domain.ErrFileTooLarge):
		return "Файл слишком большой. Максимальный размер — 10 MB."
	case errors.Is(err, domain.ErrInvalidFile):
		return "Неверный формат файла. Поддерживаются JPEG, PNG и WebP."
	default:
		return "Не удалось загрузить аватарку."
	}
}
