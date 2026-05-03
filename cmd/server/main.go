package main

import (
	"github.com/Dyuzhovsergey/gophprofile/internal/config"
	"github.com/Dyuzhovsergey/gophprofile/internal/logger"
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

	log.Info(
		"GophProfile server started",
		zap.String("address", cfg.Address),
		zap.String("log_level", cfg.LogLevel),
	)
}
