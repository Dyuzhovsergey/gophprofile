package config

import (
	"os"
	"strconv"
	"strings"
)

const (
	defaultRateLimitEnabled           = true
	defaultRateLimitRequestsPerSecond = 20.0
	defaultRateLimitBurst             = 40

	envRateLimitEnabled           = "GOPHPROFILE_RATE_LIMIT_ENABLED"
	envRateLimitRequestsPerSecond = "GOPHPROFILE_RATE_LIMIT_REQUESTS_PER_SECOND"
	envRateLimitBurst             = "GOPHPROFILE_RATE_LIMIT_BURST"
)

// RateLimitConfig хранит настройки ограничения частоты HTTP-запросов.
type RateLimitConfig struct {
	Enabled           bool
	RequestsPerSecond float64
	Burst             int
}

// LoadRateLimit загружает настройки ограничения частоты запросов
// из переменных окружения.
func LoadRateLimit() RateLimitConfig {
	return RateLimitConfig{
		Enabled: getBoolEnv(
			envRateLimitEnabled,
			defaultRateLimitEnabled,
		),
		RequestsPerSecond: getPositiveFloat64Env(
			envRateLimitRequestsPerSecond,
			defaultRateLimitRequestsPerSecond,
		),
		Burst: getPositiveIntEnv(
			envRateLimitBurst,
			defaultRateLimitBurst,
		),
	}
}

// getPositiveFloat64Env возвращает положительное float64-значение
// переменной окружения или значение по умолчанию.
func getPositiveFloat64Env(
	key string,
	defaultValue float64,
) float64 {
	rawValue := strings.TrimSpace(os.Getenv(key))
	if rawValue == "" {
		return defaultValue
	}

	value, err := strconv.ParseFloat(rawValue, 64)
	if err != nil || value <= 0 {
		return defaultValue
	}

	return value
}

// getPositiveIntEnv возвращает положительное int-значение
// переменной окружения или значение по умолчанию.
func getPositiveIntEnv(
	key string,
	defaultValue int,
) int {
	rawValue := strings.TrimSpace(os.Getenv(key))
	if rawValue == "" {
		return defaultValue
	}

	value, err := strconv.Atoi(rawValue)
	if err != nil || value <= 0 {
		return defaultValue
	}

	return value
}
