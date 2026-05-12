package handlers

import (
	"context"
	"fmt"
	"html/template"
	"net/http"
	"net/url"
	"strings"

	"github.com/Dyuzhovsergey/gophprofile/internal/domain"
	"github.com/Dyuzhovsergey/gophprofile/internal/services"
	"github.com/go-chi/chi/v5"
)

var galleryTemplate = template.Must(template.New("gallery").Parse(`<!doctype html>
<html lang="ru">
<head>
	<meta charset="utf-8">
	<title>GophProfile gallery</title>
</head>
<body>
	<h1>Галерея пользователя {{.UserID}}</h1>

	<p><a href="/web/upload">Загрузить новую аватарку</a></p>

	{{if .UploadedID}}
	<p>Загружена аватарка: {{.UploadedID}}</p>
	{{end}}

	{{if .Avatars}}
	<ul>
		{{range .Avatars}}
		<li>
			<h2>{{.FileName}}</h2>

			<img
				src="/api/v1/avatars/{{.ID}}?size=100x100"
				width="100"
				height="100"
				alt="avatar"
			>

			<p>ID: {{.ID}}</p>
			<p>MIME-type: {{.MIMEType}}</p>
			<p>Size: {{.SizeBytes}} bytes</p>
			<p>Dimensions: {{.Width}}x{{.Height}}</p>

			<p>
				<a href="/api/v1/avatars/{{.ID}}" target="_blank">original</a> |
				<a href="/api/v1/avatars/{{.ID}}?size=100x100" target="_blank">100x100</a> |
				<a href="/api/v1/avatars/{{.ID}}?size=300x300" target="_blank">300x300</a> |
				<a href="/api/v1/avatars/{{.ID}}/metadata" target="_blank">metadata</a>
			</p>
		</li>
		{{end}}
	</ul>
	{{else}}
	<p>Активных аватарок пока нет.</p>
	{{end}}
</body>
</html>`))

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

// galleryViewData содержит данные для HTML-страницы галереи.
type galleryViewData struct {
	UserID     string
	UploadedID string
	Avatars    []domain.Avatar
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

	data := galleryViewData{
		UserID:     userID,
		UploadedID: strings.TrimSpace(r.URL.Query().Get("uploaded")),
		Avatars:    avatars,
	}

	if err := galleryTemplate.Execute(w, data); err != nil {
		http.Error(w, "failed to render gallery", http.StatusInternalServerError)
		return
	}
}
