// Package config содержит загрузку и хранение конфигурации приложения GophProfile.
package config

import (
	"os"
	"strconv"
)

const (
	defaultServerAddress      = ":8080"
	defaultLogLevel           = "info"
	defaultMaxUploadSizeBytes = 10 * 1024 * 1024

	envServerAddress      = "GOPHPROFILE_SERVER_ADDRESS"
	envLogLevel           = "GOPHPROFILE_LOG_LEVEL"
	envDatabaseDSN        = "GOPHPROFILE_DATABASE_DSN"
	envMaxUploadSizeBytes = "GOPHPROFILE_MAX_UPLOAD_SIZE_BYTES"
)

// ServerConfig хранит настройки HTTP-сервера.
type ServerConfig struct {
	Address            string
	LogLevel           string
	DatabaseDSN        string
	MaxUploadSizeBytes int64
	S3                 S3Config
}

// LoadServer загружает конфигурацию HTTP-сервера из переменных окружения.
func LoadServer() ServerConfig {
	return ServerConfig{
		Address:            getEnv(envServerAddress, defaultServerAddress),
		LogLevel:           getEnv(envLogLevel, defaultLogLevel),
		DatabaseDSN:        os.Getenv(envDatabaseDSN),
		MaxUploadSizeBytes: getInt64Env(envMaxUploadSizeBytes, defaultMaxUploadSizeBytes),
		S3:                 LoadS3(),
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
