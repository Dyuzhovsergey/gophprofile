package handlers

import (
	"context"
	"fmt"
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
}

// NewWebHandler создаёт обработчик веб-интерфейса.
func NewWebHandler(avatarService WebAvatarManager, maxUploadSizeBytes int64) *WebHandler {
	if maxUploadSizeBytes <= 0 {
		maxUploadSizeBytes = services.DefaultMaxUploadSizeBytes
	}

	return &WebHandler{
		avatarService:      avatarService,
		maxUploadSizeBytes: maxUploadSizeBytes,
	}
}

// UploadPage отдаёт готовую страницу загрузки из web/static/index.html.
func (h *WebHandler) UploadPage(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "web/static/index.html")
}

// Upload обрабатывает загрузку аватарки через web endpoint.
func (h *WebHandler) Upload(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, h.maxUploadSizeBytes)

	if err := r.ParseMultipartForm(h.maxUploadSizeBytes); err != nil {
		http.Error(w, "failed to parse multipart form", http.StatusBadRequest)
		return
	}

	userID := strings.TrimSpace(r.FormValue("user_id"))
	if userID == "" {
		userID = strings.TrimSpace(r.FormValue("userId"))
	}

	if userID == "" {
		http.Error(w, "user_id is required", http.StatusBadRequest)
		return
	}

	file, fileHeader, err := formFileByNames(r, "file", "image")
	if err != nil {
		http.Error(w, "file is required", http.StatusBadRequest)
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
		http.Error(w, "failed to upload avatar", http.StatusBadRequest)
		return
	}

	redirectURL := fmt.Sprintf(
		"/web/gallery/%s?uploaded=%s",
		url.PathEscape(userID),
		url.QueryEscape(avatar.ID),
	)

	http.Redirect(w, r, redirectURL, http.StatusSeeOther)
}

// Gallery отображает простую HTML-галерею активных аватарок пользователя.
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

	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	fmt.Fprintf(w, "<!doctype html>")
	fmt.Fprintf(w, "<html lang=\"ru\">")
	fmt.Fprintf(w, "<head>")
	fmt.Fprintf(w, "<meta charset=\"utf-8\">")
	fmt.Fprintf(w, "<title>GophProfile gallery</title>")
	fmt.Fprintf(w, "</head>")
	fmt.Fprintf(w, "<body>")
	fmt.Fprintf(w, "<h1>Галерея пользователя %s</h1>", userID)
	fmt.Fprintf(w, "<p><a href=\"/web/upload\">Загрузить новую аватарку</a></p>")

	uploadedID := strings.TrimSpace(r.URL.Query().Get("uploaded"))
	if uploadedID != "" {
		fmt.Fprintf(w, "<p>Загружена аватарка: %s</p>", uploadedID)
	}

	if len(avatars) == 0 {
		fmt.Fprintf(w, "<p>Активных аватарок пока нет.</p>")
		fmt.Fprintf(w, "</body></html>")
		return
	}

	fmt.Fprintf(w, "<ul>")

	for _, avatar := range avatars {
		fmt.Fprintf(w, "<li>")
		fmt.Fprintf(w, "<h2>%s</h2>", avatar.FileName)
		fmt.Fprintf(
			w,
			"<img src=\"/api/v1/avatars/%s?size=100x100\" width=\"100\" height=\"100\" alt=\"avatar\">",
			avatar.ID,
		)
		fmt.Fprintf(w, "<p>ID: %s</p>", avatar.ID)
		fmt.Fprintf(w, "<p>MIME-type: %s</p>", avatar.MIMEType)
		fmt.Fprintf(w, "<p>Size: %d bytes</p>", avatar.SizeBytes)
		fmt.Fprintf(w, "<p>Dimensions: %dx%d</p>", avatar.Width, avatar.Height)
		fmt.Fprintf(w, "<p>")
		fmt.Fprintf(w, "<a href=\"/api/v1/avatars/%s\" target=\"_blank\">original</a> | ", avatar.ID)
		fmt.Fprintf(w, "<a href=\"/api/v1/avatars/%s?size=100x100\" target=\"_blank\">100x100</a> | ", avatar.ID)
		fmt.Fprintf(w, "<a href=\"/api/v1/avatars/%s?size=300x300\" target=\"_blank\">300x300</a> | ", avatar.ID)
		fmt.Fprintf(w, "<a href=\"/api/v1/avatars/%s/metadata\" target=\"_blank\">metadata</a>", avatar.ID)
		fmt.Fprintf(w, "</p>")
		fmt.Fprintf(w, "</li>")
	}

	fmt.Fprintf(w, "</ul>")
	fmt.Fprintf(w, "</body></html>")
}
