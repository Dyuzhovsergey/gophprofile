package main

import (
	"errors"
	"net/http"
	"time"

	"github.com/Dyuzhovsergey/gophprofile/internal/config"
	"github.com/Dyuzhovsergey/gophprofile/internal/handlers"
	"github.com/Dyuzhovsergey/gophprofile/internal/logger"
	"github.com/Dyuzhovsergey/gophprofile/internal/middleware"
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

	router := chi.NewRouter()
	router.Use(middleware.Recover(log))
	router.Use(middleware.RequestLogger(log))

	router.Get("/health", handlers.Health)

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
