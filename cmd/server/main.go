package main

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/Dyuzhovsergey/gophprofile/internal/config"
	"github.com/Dyuzhovsergey/gophprofile/internal/handlers"
	"github.com/Dyuzhovsergey/gophprofile/internal/logger"
	"github.com/Dyuzhovsergey/gophprofile/internal/middleware"
	"github.com/Dyuzhovsergey/gophprofile/internal/repository/postgres"
	s3storage "github.com/Dyuzhovsergey/gophprofile/internal/repository/s3"
	"github.com/Dyuzhovsergey/gophprofile/internal/services"
	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"
)

func main() {
	cfg := config.LoadServer()

	log, err := logger.Init(cfg.LogLevel)
	if err != nil {
		panic(err)
	}
	defer func() {
		_ = log.Sync()
	}()

	db, err := postgres.NewPool(context.Background(), cfg.DatabaseDSN)
	if err != nil {
		log.Fatal("failed to connect to postgres", zap.Error(err))
	}
	defer db.Close()

	log.Info("connected to postgres")

	avatarRepository := postgres.NewAvatarRepository(db)

	avatarStorage, err := s3storage.NewClient(context.Background(), cfg.S3)
	if err != nil {
		log.Fatal("failed to create s3 storage client", zap.Error(err))
	}

	avatarService := services.NewAvatarService(
		avatarRepository,
		avatarStorage,
		cfg.MaxUploadSizeBytes,
	)

	avatarHandler := handlers.NewAvatarHandler(
		avatarService,
		cfg.MaxUploadSizeBytes,
	)

	log.Info("s3 storage client created")

	router := chi.NewRouter()
	router.Use(middleware.Recover(log))
	router.Use(middleware.RequestLogger(log))

	healthHandler := handlers.NewHealthHandler(db)
	router.Get("/health", healthHandler.Handle)

	router.Route("/api/v1", func(r chi.Router) {
		r.Group(func(r chi.Router) {
			r.Use(middleware.RequireUserID)

			r.Post("/avatars", avatarHandler.Upload)
		})
	})

	server := &http.Server{
		Addr:              cfg.Address,
		Handler:           router,
		ReadHeaderTimeout: 5 * time.Second,
	}

	log.Info(
		"GophProfile server starting",
		zap.String("address", cfg.Address),
		zap.String("log_level", cfg.LogLevel),
	)

	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatal("GophProfile server stopped with error", zap.Error(err))
	}
}
