package main

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Dyuzhovsergey/gophprofile/internal/broker/rabbitmq"
	"github.com/Dyuzhovsergey/gophprofile/internal/config"
	"github.com/Dyuzhovsergey/gophprofile/internal/logger"
	observabilitylogging "github.com/Dyuzhovsergey/gophprofile/internal/observability/logging"
	observabilitytracing "github.com/Dyuzhovsergey/gophprofile/internal/observability/tracing"
	"github.com/Dyuzhovsergey/gophprofile/internal/repository/postgres"
	s3storage "github.com/Dyuzhovsergey/gophprofile/internal/repository/s3"
	"github.com/Dyuzhovsergey/gophprofile/internal/services"
	avatarworker "github.com/Dyuzhovsergey/gophprofile/internal/worker"
)

func main() {
	cfg := config.LoadWorker()

	log, err := logger.Init(
		cfg.LogLevel,
		observabilitylogging.ServiceNameWorker,
		observabilitylogging.DefaultEnvironment,
	)
	if err != nil {
		panic(err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM, syscall.SIGINT)
	defer stop()

	tracingShutdown, err := observabilitytracing.InitProvider(
		ctx,
		observabilitytracing.NewConfig(
			cfg.Tracing.ServiceName,
			cfg.Tracing.ExporterEndpoint,
			cfg.Tracing.Enabled,
		),
	)
	if err != nil {
		log.Error("failed to initialize tracing", logger.Err(err))
		os.Exit(1)
	}

	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := tracingShutdown(shutdownCtx); err != nil {
			log.Error("failed to shutdown tracing", logger.Err(err))
		}
	}()

	log.Info(
		"tracing initialized",
		slog.Bool("enabled", cfg.Tracing.Enabled),
		slog.String("service_name", cfg.Tracing.ServiceName),
		slog.String("exporter_endpoint", cfg.Tracing.ExporterEndpoint),
	)

	db, err := postgres.NewPool(ctx, cfg.DatabaseDSN)
	if err != nil {
		log.Error("failed to connect to postgres", logger.Err(err))
		os.Exit(1)
	}
	defer db.Close()

	log.Info("connected to postgres")

	avatarRepository := postgres.NewAvatarRepository(db)

	avatarStorage, err := s3storage.NewClient(ctx, cfg.S3)
	if err != nil {
		log.Error("failed to create s3 storage client", logger.Err(err))
		os.Exit(1)
	}

	imageService := services.NewImageService()

	log.Info("s3 storage client created")

	consumer, err := rabbitmq.NewConsumer(cfg.RabbitMQ)
	if err != nil {
		log.Error("failed to create rabbitmq consumer", logger.Err(err))
		os.Exit(1)
	}
	defer func() {
		if err := consumer.Close(); err != nil {
			log.Error("failed to close rabbitmq consumer", logger.Err(err))
		}
	}()

	processor := avatarworker.NewAvatarProcessor(
		log,
		avatarRepository,
		avatarStorage,
		imageService,
	)

	log.Info(
		"GophProfile worker started",
		slog.String("log_level", cfg.LogLevel),
		slog.String("upload_queue", cfg.RabbitMQ.UploadQueue),
	)

	if err := consumer.ConsumeAvatarEvents(
		ctx,
		processor.HandleAvatarUploaded,
		processor.HandleAvatarDeleted,
	); err != nil && !errors.Is(err, context.Canceled) {
		log.Error("failed to consume avatar events", logger.Err(err))
		os.Exit(1)
	}

	log.Info("GophProfile worker stopped")
}
