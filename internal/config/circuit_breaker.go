package config

import (
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	defaultCircuitBreakerEnabled          = true
	defaultCircuitBreakerFailureThreshold = uint32(5)
	defaultCircuitBreakerOpenTimeout      = 30 * time.Second

	envCircuitBreakerEnabled          = "GOPHPROFILE_CIRCUIT_BREAKER_ENABLED"
	envCircuitBreakerFailureThreshold = "GOPHPROFILE_CIRCUIT_BREAKER_FAILURE_THRESHOLD"
	envCircuitBreakerOpenTimeout      = "GOPHPROFILE_CIRCUIT_BREAKER_OPEN_TIMEOUT"
)

// CircuitBreakerConfig хранит общие настройки Circuit Breaker.
type CircuitBreakerConfig struct {
	Enabled          bool
	FailureThreshold uint32
	OpenTimeout      time.Duration
}

// LoadCircuitBreaker загружает настройки Circuit Breaker
// из переменных окружения.
func LoadCircuitBreaker() CircuitBreakerConfig {
	return CircuitBreakerConfig{
		Enabled: getBoolEnv(
			envCircuitBreakerEnabled,
			defaultCircuitBreakerEnabled,
		),
		FailureThreshold: getPositiveUint32Env(
			envCircuitBreakerFailureThreshold,
			defaultCircuitBreakerFailureThreshold,
		),
		OpenTimeout: getPositiveDurationEnv(
			envCircuitBreakerOpenTimeout,
			defaultCircuitBreakerOpenTimeout,
		),
	}
}

// getPositiveUint32Env возвращает положительное uint32-значение
// переменной окружения или значение по умолчанию.
func getPositiveUint32Env(
	key string,
	defaultValue uint32,
) uint32 {
	rawValue := strings.TrimSpace(os.Getenv(key))
	if rawValue == "" {
		return defaultValue
	}

	value, err := strconv.ParseUint(rawValue, 10, 32)
	if err != nil || value == 0 {
		return defaultValue
	}

	return uint32(value)
}

// getPositiveDurationEnv возвращает положительную длительность
// переменной окружения или значение по умолчанию.
func getPositiveDurationEnv(
	key string,
	defaultValue time.Duration,
) time.Duration {
	rawValue := strings.TrimSpace(os.Getenv(key))
	if rawValue == "" {
		return defaultValue
	}

	value, err := time.ParseDuration(rawValue)
	if err != nil || value <= 0 {
		return defaultValue
	}

	return value
}
