package main

import (
	"context"
	"errors"
	"os"
	"os/signal"
	"syscall"

	"github.com/Dyuzhovsergey/gophprofile/internal/broker/rabbitmq"
	"github.com/Dyuzhovsergey/gophprofile/internal/config"
	"github.com/Dyuzhovsergey/gophprofile/internal/logger"
	"github.com/Dyuzhovsergey/gophprofile/internal/repository/postgres"
	s3storage "github.com/Dyuzhovsergey/gophprofile/internal/repository/s3"
	"github.com/Dyuzhovsergey/gophprofile/internal/services"
	avatarworker "github.com/Dyuzhovsergey/gophprofile/internal/worker"
	"go.uber.org/zap"
)

func main() {
	cfg := config.LoadWorker()

	log, err := logger.Init(cfg.LogLevel)
	if err != nil {
		panic(err)
	}
	defer func() {
		_ = log.Sync()
	}()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM, syscall.SIGINT)
	defer stop()

	db, err := postgres.NewPool(ctx, cfg.DatabaseDSN)
	if err != nil {
		log.Fatal("failed to connect to postgres", zap.Error(err))
	}
	defer db.Close()

	log.Info("connected to postgres")

	avatarRepository := postgres.NewAvatarRepository(db)

	avatarStorage, err := s3storage.NewClient(ctx, cfg.S3)
	if err != nil {
		log.Fatal("failed to create s3 storage client", zap.Error(err))
	}

	imageService := services.NewImageService()

	log.Info("s3 storage client created")

	consumer, err := rabbitmq.NewConsumer(cfg.RabbitMQ)
	if err != nil {
		log.Fatal("failed to create rabbitmq consumer", zap.Error(err))
	}
	defer func() {
		if err := consumer.Close(); err != nil {
			log.Error("failed to close rabbitmq consumer", zap.Error(err))
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
		zap.String("log_level", cfg.LogLevel),
		zap.String("upload_queue", cfg.RabbitMQ.UploadQueue),
	)

	if err := consumer.ConsumeAvatarUploaded(ctx, processor.HandleAvatarUploaded); err != nil &&
		!errors.Is(err, context.Canceled) {
		log.Fatal("failed to consume avatar uploaded events", zap.Error(err))
	}

	log.Info("GophProfile worker stopped")
}
