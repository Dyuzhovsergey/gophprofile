package main

import (
	"context"
	"errors"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/Dyuzhovsergey/gophprofile/internal/broker/rabbitmq"
	"github.com/Dyuzhovsergey/gophprofile/internal/config"
	"github.com/Dyuzhovsergey/gophprofile/internal/handlers"
	"github.com/Dyuzhovsergey/gophprofile/internal/logger"
	"github.com/Dyuzhovsergey/gophprofile/internal/outbox"
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

	ctx, stop := signal.NotifyContext(
		context.Background(),
		os.Interrupt,
		syscall.SIGTERM,
		syscall.SIGINT,
	)
	defer stop()

	db, err := postgres.NewPool(ctx, cfg.DatabaseDSN)
	if err != nil {
		log.Fatal("failed to connect to postgres", zap.Error(err))
	}
	defer db.Close()

	log.Info("connected to postgres")

	avatarRepository := postgres.NewAvatarRepository(db)
	outboxRepository := postgres.NewOutboxRepository(db)

	avatarStorage, err := s3storage.NewClient(ctx, cfg.S3)
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

	outboxDispatcher := outbox.NewDispatcher(
		outboxRepository,
		avatarEventPublisher,
		log,
	)

	go outboxDispatcher.Run(ctx)

	log.Info("outbox dispatcher started")

	avatarService := services.NewAvatarService(
		avatarRepository,
		avatarStorage,
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
		ReadHeaderTimeout: cfg.ReadHeaderTimeout,
		ReadTimeout:       cfg.ReadTimeout,
		WriteTimeout:      cfg.WriteTimeout,
		IdleTimeout:       cfg.IdleTimeout,
	}

	log.Info(
		"GophProfile server starting",
		zap.String("address", cfg.Address),
		zap.String("log_level", cfg.LogLevel),
	)

	serverErr := make(chan error, 1)

	go func() {
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErr <- err
			return
		}

		serverErr <- nil
	}()

	select {
	case <-ctx.Done():
		log.Info("shutdown signal received")

	case err := <-serverErr:
		if err != nil {
			log.Fatal("GophProfile server stopped with error", zap.Error(err))
		}

		log.Info("GophProfile server stopped")
		return
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.GracefulShutdownTimeout)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Fatal("failed to shutdown GophProfile server gracefully", zap.Error(err))
	}

	if err := <-serverErr; err != nil {
		log.Fatal("GophProfile server stopped with error", zap.Error(err))
	}

	log.Info("GophProfile server stopped gracefully")
}
