package config

import "testing"

func TestLoadRateLimit_Defaults(t *testing.T) {
	t.Setenv(envRateLimitEnabled, "")
	t.Setenv(envRateLimitRequestsPerSecond, "")
	t.Setenv(envRateLimitBurst, "")

	cfg := LoadRateLimit()

	if cfg.Enabled != defaultRateLimitEnabled {
		t.Fatalf(
			"unexpected enabled value: got %v, want %v",
			cfg.Enabled,
			defaultRateLimitEnabled,
		)
	}

	if cfg.RequestsPerSecond != defaultRateLimitRequestsPerSecond {
		t.Fatalf(
			"unexpected requests per second: got %v, want %v",
			cfg.RequestsPerSecond,
			defaultRateLimitRequestsPerSecond,
		)
	}

	if cfg.Burst != defaultRateLimitBurst {
		t.Fatalf(
			"unexpected burst: got %d, want %d",
			cfg.Burst,
			defaultRateLimitBurst,
		)
	}
}

func TestLoadRateLimit_FromEnv(t *testing.T) {
	t.Setenv(envRateLimitEnabled, "false")
	t.Setenv(envRateLimitRequestsPerSecond, "12.5")
	t.Setenv(envRateLimitBurst, "25")

	cfg := LoadRateLimit()

	if cfg.Enabled {
		t.Fatal("expected rate limiting to be disabled")
	}

	if cfg.RequestsPerSecond != 12.5 {
		t.Fatalf(
			"unexpected requests per second: got %v, want 12.5",
			cfg.RequestsPerSecond,
		)
	}

	if cfg.Burst != 25 {
		t.Fatalf(
			"unexpected burst: got %d, want 25",
			cfg.Burst,
		)
	}
}

func TestLoadRateLimit_InvalidValuesUseDefaults(t *testing.T) {
	t.Setenv(envRateLimitEnabled, "invalid")
	t.Setenv(envRateLimitRequestsPerSecond, "-10")
	t.Setenv(envRateLimitBurst, "0")

	cfg := LoadRateLimit()

	if cfg.Enabled != defaultRateLimitEnabled {
		t.Fatalf(
			"unexpected enabled value: got %v, want %v",
			cfg.Enabled,
			defaultRateLimitEnabled,
		)
	}

	if cfg.RequestsPerSecond != defaultRateLimitRequestsPerSecond {
		t.Fatalf(
			"unexpected requests per second: got %v, want %v",
			cfg.RequestsPerSecond,
			defaultRateLimitRequestsPerSecond,
		)
	}

	if cfg.Burst != defaultRateLimitBurst {
		t.Fatalf(
			"unexpected burst: got %d, want %d",
			cfg.Burst,
			defaultRateLimitBurst,
		)
	}
}
