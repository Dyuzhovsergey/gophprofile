package handlers

import (
	"log/slog"
	"net/http"

	"github.com/Dyuzhovsergey/gophprofile/internal/middleware"
	observabilitylogging "github.com/Dyuzhovsergey/gophprofile/internal/observability/logging"
	observabilitymetrics "github.com/Dyuzhovsergey/gophprofile/internal/observability/metrics"
	"github.com/go-chi/chi/v5"
)

// NewRouter создаёт HTTP-router приложения GophProfile.
func NewRouter(
	log *slog.Logger,
	healthHandler *HealthHandler,
	avatarHandler *AvatarHandler,
	webHandler *WebHandler,
) http.Handler {
	router := chi.NewRouter()

	router.Use(middleware.Tracing(observabilitylogging.ServiceNameServer, router))
	router.Use(middleware.Recover(log))
	router.Use(middleware.RequestLogger(log))
	router.Use(middleware.CORS)

	router.Get("/health", healthHandler.Handle)
	router.Handle("/metrics", observabilitymetrics.Handler())
	router.Get("/", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/web/upload", http.StatusSeeOther)
	})

	router.Handle(
		"/web/static/*",
		http.StripPrefix("/web/static/", http.FileServer(http.Dir("web/static"))),
	)

	router.Get("/web/upload", webHandler.UploadPage)
	router.Post("/web/upload", webHandler.Upload)
	router.Get("/web/gallery/{user_id}", webHandler.Gallery)

	router.Route("/api/v1", func(r chi.Router) {
		r.Get("/avatars/{avatar_id}", avatarHandler.GetByID)
		r.Get("/avatars/{avatar_id}/metadata", avatarHandler.GetMetadata)

		r.Get("/users/{user_id}/avatar", avatarHandler.GetCurrentByUserID)
		r.Get("/users/{user_id}/avatars", avatarHandler.ListByUserID)

		r.Group(func(r chi.Router) {
			r.Use(middleware.RequireUserID)

			r.Post("/avatars", avatarHandler.Upload)
			r.Delete("/avatars/{avatar_id}", avatarHandler.DeleteByID)
			r.Delete("/users/{user_id}/avatar", avatarHandler.DeleteCurrentByUserID)
		})
	})

	return router
}
