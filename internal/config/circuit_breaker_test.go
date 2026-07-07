package config

import (
	"testing"
	"time"
)

func TestLoadCircuitBreaker_Defaults(t *testing.T) {
	t.Setenv(envCircuitBreakerEnabled, "")
	t.Setenv(envCircuitBreakerFailureThreshold, "")
	t.Setenv(envCircuitBreakerOpenTimeout, "")

	cfg := LoadCircuitBreaker()

	if cfg.Enabled != defaultCircuitBreakerEnabled {
		t.Fatalf(
			"unexpected enabled value: got %v, want %v",
			cfg.Enabled,
			defaultCircuitBreakerEnabled,
		)
	}

	if cfg.FailureThreshold != defaultCircuitBreakerFailureThreshold {
		t.Fatalf(
			"unexpected threshold: got %d, want %d",
			cfg.FailureThreshold,
			defaultCircuitBreakerFailureThreshold,
		)
	}

	if cfg.OpenTimeout != defaultCircuitBreakerOpenTimeout {
		t.Fatalf(
			"unexpected open timeout: got %s, want %s",
			cfg.OpenTimeout,
			defaultCircuitBreakerOpenTimeout,
		)
	}
}

func TestLoadCircuitBreaker_FromEnv(t *testing.T) {
	t.Setenv(envCircuitBreakerEnabled, "false")
	t.Setenv(envCircuitBreakerFailureThreshold, "3")
	t.Setenv(envCircuitBreakerOpenTimeout, "45s")

	cfg := LoadCircuitBreaker()

	if cfg.Enabled {
		t.Fatal("expected circuit breaker to be disabled")
	}

	if cfg.FailureThreshold != 3 {
		t.Fatalf(
			"unexpected threshold: got %d, want 3",
			cfg.FailureThreshold,
		)
	}

	if cfg.OpenTimeout != 45*time.Second {
		t.Fatalf(
			"unexpected open timeout: got %s, want 45s",
			cfg.OpenTimeout,
		)
	}
}

func TestLoadCircuitBreaker_InvalidValuesUseDefaults(
	t *testing.T,
) {
	t.Setenv(envCircuitBreakerEnabled, "invalid")
	t.Setenv(envCircuitBreakerFailureThreshold, "0")
	t.Setenv(envCircuitBreakerOpenTimeout, "invalid")

	cfg := LoadCircuitBreaker()

	if cfg.Enabled != defaultCircuitBreakerEnabled {
		t.Fatalf(
			"unexpected enabled value: got %v, want %v",
			cfg.Enabled,
			defaultCircuitBreakerEnabled,
		)
	}

	if cfg.FailureThreshold != defaultCircuitBreakerFailureThreshold {
		t.Fatalf(
			"unexpected threshold: got %d, want %d",
			cfg.FailureThreshold,
			defaultCircuitBreakerFailureThreshold,
		)
	}

	if cfg.OpenTimeout != defaultCircuitBreakerOpenTimeout {
		t.Fatalf(
			"unexpected open timeout: got %s, want %s",
			cfg.OpenTimeout,
			defaultCircuitBreakerOpenTimeout,
		)
	}
}
