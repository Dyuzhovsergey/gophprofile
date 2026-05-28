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

	"github.com/Dyuzhovsergey/gophprofile/internal/broker/rabbitmq"
	"github.com/Dyuzhovsergey/gophprofile/internal/config"
	"github.com/Dyuzhovsergey/gophprofile/internal/handlers"
	"github.com/Dyuzhovsergey/gophprofile/internal/logger"
	observabilitylogging "github.com/Dyuzhovsergey/gophprofile/internal/observability/logging"
	observabilitymetrics "github.com/Dyuzhovsergey/gophprofile/internal/observability/metrics"
	observabilitytracing "github.com/Dyuzhovsergey/gophprofile/internal/observability/tracing"
	"github.com/Dyuzhovsergey/gophprofile/internal/outbox"
	"github.com/Dyuzhovsergey/gophprofile/internal/repository/postgres"
	s3storage "github.com/Dyuzhovsergey/gophprofile/internal/repository/s3"
	"github.com/Dyuzhovsergey/gophprofile/internal/services"
)

func main() {
	cfg := config.LoadServer()

	log, err := logger.Init(
		cfg.LogLevel,
		observabilitylogging.ServiceNameServer,
		observabilitylogging.DefaultEnvironment,
	)
	if err != nil {
		fmt.Fprintln(os.Stderr, "init logger:", err)
		os.Exit(1)
	}

	appMetrics := observabilitymetrics.New()

	ctx, stop := signal.NotifyContext(
		context.Background(),
		os.Interrupt,
		syscall.SIGTERM,
		syscall.SIGINT,
	)
	defer stop()

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
		shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.GracefulShutdownTimeout)
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

	log.Info("connected to postgres")

	avatarRepository := postgres.NewAvatarRepository(db)
	outboxRepository := postgres.NewOutboxRepository(db)

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

	avatarEventPublisher, err := rabbitmq.NewPublisher(cfg.RabbitMQ)
	if err != nil {
		log.LogAttrs(
			ctx,
			slog.LevelError,
			"failed to create rabbitmq publisher",
			observabilitylogging.ErrorAttrs(
				ctx,
				observabilitylogging.ComponentRabbitMQ,
				"rabbitmq.create_publisher",
				err,
			)...,
		)
		os.Exit(1)
	}
	defer func() {
		if err := avatarEventPublisher.Close(); err != nil {
			log.Error("failed to close rabbitmq publisher", logger.Err(err))
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
		log,
	)
	avatarService.WithAvatarMetrics(appMetrics.Avatar)

	avatarHandler := handlers.NewAvatarHandler(
		avatarService,
		cfg.MaxUploadSizeBytes,
		log,
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
	router := handlers.NewRouter(log, healthHandler, avatarHandler, appMetrics)

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
		slog.String("address", cfg.Address),
		slog.String("log_level", cfg.LogLevel),
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
			log.LogAttrs(
				ctx,
				slog.LevelError,
				"GophProfile server stopped with error",
				observabilitylogging.ErrorAttrs(
					ctx,
					observabilitylogging.ComponentApp,
					"server.listen",
					err,
				)...,
			)
			os.Exit(1)
		}

		log.Info("GophProfile server stopped")
		return
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.GracefulShutdownTimeout)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Error("failed to shutdown GophProfile server gracefully", logger.Err(err))
		os.Exit(1)
	}

	if err := <-serverErr; err != nil {
		log.Error("GophProfile server stopped with error", logger.Err(err))
		os.Exit(1)
	}

	log.Info("GophProfile server stopped gracefully")
}
