package main

import (
	"github.com/Dyuzhovsergey/gophprofile/internal/config"
	"github.com/Dyuzhovsergey/gophprofile/internal/logger"
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

	log.Info(
		"GophProfile worker started",
		zap.String("log_level", cfg.LogLevel),
	)
}
