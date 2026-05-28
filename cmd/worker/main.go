package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Dyuzhovsergey/gophprofile/internal/broker/rabbitmq"
	"github.com/Dyuzhovsergey/gophprofile/internal/config"
	"github.com/Dyuzhovsergey/gophprofile/internal/logger"
	observabilitylogging "github.com/Dyuzhovsergey/gophprofile/internal/observability/logging"
	observabilitymetrics "github.com/Dyuzhovsergey/gophprofile/internal/observability/metrics"
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
		fmt.Fprintln(os.Stderr, "init logger:", err)
		os.Exit(1)
	}

	appMetrics := observabilitymetrics.New()

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

	metricsMux := http.NewServeMux()
	metricsMux.Handle("/metrics", appMetrics.Handler())

	metricsServer := &http.Server{
		Addr:              cfg.MetricsAddress,
		Handler:           metricsMux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		log.Info(
			"worker metrics server starting",
			slog.String("address", cfg.MetricsAddress),
		)

		if err := metricsServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Error("worker metrics server stopped with error", logger.Err(err))
		}
	}()

	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := metricsServer.Shutdown(shutdownCtx); err != nil {
			log.Error("failed to shutdown worker metrics server", logger.Err(err))
		}
	}()

	db, err := postgres.NewPool(ctx, cfg.DatabaseDSN)
	if err != nil {
		log.LogAttrs(
			ctx,
			slog.LevelError,
			"failed to connect to postgres",
			observabilitylogging.ErrorAttrs(
				ctx,
				observabilitylogging.ComponentPostgres,
				"postgres.connect",
				err,
			)...,
		)
		os.Exit(1)
	}
	defer db.Close()

	log.Info("connected to postgres")

	avatarRepository := postgres.NewAvatarRepository(db)

	avatarStorage, err := s3storage.NewClient(ctx, cfg.S3)
	if err != nil {
		log.LogAttrs(
			ctx,
			slog.LevelError,
			"failed to create s3 storage client",
			observabilitylogging.ErrorAttrs(
				ctx,
				observabilitylogging.ComponentS3,
				"s3.create_client",
				err,
			)...,
		)
		os.Exit(1)
	}

	imageService := services.NewImageService()

	log.Info("s3 storage client created")

	consumer, err := rabbitmq.NewConsumer(cfg.RabbitMQ)
	if err != nil {
		log.LogAttrs(
			ctx,
			slog.LevelError,
			"failed to create rabbitmq consumer",
			observabilitylogging.ErrorAttrs(
				ctx,
				observabilitylogging.ComponentRabbitMQ,
				"rabbitmq.create_consumer",
				err,
			)...,
		)
		os.Exit(1)
	}
	defer func() {
		if err := consumer.Close(); err != nil {
			log.Error("failed to close rabbitmq consumer", logger.Err(err))
		}
	}()

	consumer.WithWorkerMetrics(appMetrics.Worker)

	processor := avatarworker.NewAvatarProcessor(
		log,
		avatarRepository,
		avatarStorage,
		imageService,
	)

	processor.WithAvatarMetrics(appMetrics.Avatar)
	consumer.WithLogger(log)

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
		log.LogAttrs(
			ctx,
			slog.LevelError,
			"failed to consume avatar events",
			observabilitylogging.ErrorAttrs(
				ctx,
				observabilitylogging.ComponentWorker,
				"worker.consume_avatar_events",
				err,
			)...,
		)
		os.Exit(1)
	}

	log.Info("GophProfile worker stopped")
}
