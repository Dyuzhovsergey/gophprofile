// Package config содержит загрузку и хранение конфигурации приложения GophProfile.
package config

import (
	"os"
	"strconv"
	"time"
)

const (
	defaultServerAddress      = ":8080"
	defaultLogLevel           = "info"
	defaultMaxUploadSizeBytes = 10 * 1024 * 1024

	defaultGracefulShutdownTimeout = 10 * time.Second
	defaultServerReadHeaderTimeout = 5 * time.Second
	defaultServerReadTimeout       = 30 * time.Second
	defaultServerWriteTimeout      = 30 * time.Second
	defaultServerIdleTimeout       = 60 * time.Second

	envServerAddress      = "GOPHPROFILE_SERVER_ADDRESS"
	envLogLevel           = "GOPHPROFILE_LOG_LEVEL"
	envDatabaseDSN        = "GOPHPROFILE_DATABASE_DSN"
	envMaxUploadSizeBytes = "GOPHPROFILE_MAX_UPLOAD_SIZE_BYTES"
)

// ServerConfig хранит настройки HTTP-сервера.
// ServerConfig хранит настройки HTTP-сервера.
type ServerConfig struct {
	Address            string
	LogLevel           string
	DatabaseDSN        string
	MaxUploadSizeBytes int64

	GracefulShutdownTimeout time.Duration
	ReadHeaderTimeout       time.Duration
	ReadTimeout             time.Duration
	WriteTimeout            time.Duration
	IdleTimeout             time.Duration

	S3       S3Config
	RabbitMQ RabbitMQConfig
	Tracing  TracingConfig
}

// LoadServer загружает конфигурацию HTTP-сервера из переменных окружения.
func LoadServer() ServerConfig {
	return ServerConfig{
		Address:            getEnv(envServerAddress, defaultServerAddress),
		LogLevel:           getEnv(envLogLevel, defaultLogLevel),
		DatabaseDSN:        os.Getenv(envDatabaseDSN),
		MaxUploadSizeBytes: getInt64Env(envMaxUploadSizeBytes, defaultMaxUploadSizeBytes),

		GracefulShutdownTimeout: defaultGracefulShutdownTimeout,
		ReadHeaderTimeout:       defaultServerReadHeaderTimeout,
		ReadTimeout:             defaultServerReadTimeout,
		WriteTimeout:            defaultServerWriteTimeout,
		IdleTimeout:             defaultServerIdleTimeout,

		S3:       LoadS3(),
		RabbitMQ: LoadRabbitMQ(),
		Tracing:  LoadTracing(defaultServerServiceName),
	}
}

// getInt64Env возвращает int64-значение переменной окружения или значение по умолчанию.
func getInt64Env(key string, defaultValue int64) int64 {
	rawValue := os.Getenv(key)
	if rawValue == "" {
		return defaultValue
	}

	value, err := strconv.ParseInt(rawValue, 10, 64)
	if err != nil {
		return defaultValue
	}

	return value
}

// getEnv возвращает значение переменной окружения или значение по умолчанию.
func getEnv(key string, defaultValue string) string {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}

	return value
}
