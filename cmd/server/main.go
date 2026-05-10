package main

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/Dyuzhovsergey/gophprofile/internal/broker/rabbitmq"
	"github.com/Dyuzhovsergey/gophprofile/internal/config"
	"github.com/Dyuzhovsergey/gophprofile/internal/handlers"
	"github.com/Dyuzhovsergey/gophprofile/internal/logger"
	"github.com/Dyuzhovsergey/gophprofile/internal/repository/postgres"
	s3storage "github.com/Dyuzhovsergey/gophprofile/internal/repository/s3"
	"github.com/Dyuzhovsergey/gophprofile/internal/services"
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

	avatarEventPublisher, err := rabbitmq.NewPublisher(cfg.RabbitMQ)
	if err != nil {
		log.Fatal("failed to create rabbitmq publisher", zap.Error(err))
	}
	defer func() {
		if err := avatarEventPublisher.Close(); err != nil {
			log.Error("failed to close rabbitmq publisher", zap.Error(err))
		}
	}()

	log.Info("rabbitmq publisher created")

	avatarService := services.NewAvatarServiceWithPublisher(
		avatarRepository,
		avatarStorage,
		avatarEventPublisher,
		cfg.MaxUploadSizeBytes,
	)

	avatarHandler := handlers.NewAvatarHandler(
		avatarService,
		cfg.MaxUploadSizeBytes,
	)

	webHandler := handlers.NewWebHandler(
		avatarService,
		cfg.MaxUploadSizeBytes,
	)

	log.Info("s3 storage client created")

	healthHandler := handlers.NewHealthHandler(
		db,
		avatarStorage,
		avatarEventPublisher,
	)

	router := handlers.NewRouter(log, healthHandler, avatarHandler, webHandler)
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
