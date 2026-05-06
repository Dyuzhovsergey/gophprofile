package main

import (
	"context"

	"github.com/Dyuzhovsergey/gophprofile/internal/config"
	"github.com/Dyuzhovsergey/gophprofile/internal/logger"
	"github.com/Dyuzhovsergey/gophprofile/internal/repository/postgres"
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

	db, err := postgres.NewPool(context.Background(), cfg.DatabaseDSN)
	if err != nil {
		log.Fatal("failed to connect to postgres", zap.Error(err))
	}
	defer db.Close()

	log.Info(
		"GophProfile worker started",
		zap.String("log_level", cfg.LogLevel),
	)

	log.Info("connected to postgres")
}
