package handlers

import (
	"net/http"

	"github.com/Dyuzhovsergey/gophprofile/internal/middleware"
	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"
)

// NewRouter создаёт HTTP-router приложения GophProfile.
func NewRouter(
	log *zap.Logger,
	healthHandler *HealthHandler,
	avatarHandler *AvatarHandler,
) http.Handler {
	router := chi.NewRouter()

	router.Use(middleware.Recover(log))
	router.Use(middleware.RequestLogger(log))

	router.Get("/health", healthHandler.Handle)

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
