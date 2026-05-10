package config

import "testing"

func TestLoadServer_Defaults(t *testing.T) {
	t.Setenv(envServerAddress, "")
	t.Setenv(envLogLevel, "")
	t.Setenv(envDatabaseDSN, "")
	t.Setenv(envMaxUploadSizeBytes, "")

	cfg := LoadServer()

	if cfg.Address != defaultServerAddress {
		t.Fatalf("unexpected address: got %q, want %q", cfg.Address, defaultServerAddress)
	}

	if cfg.LogLevel != defaultLogLevel {
		t.Fatalf("unexpected log level: got %q, want %q", cfg.LogLevel, defaultLogLevel)
	}

	if cfg.DatabaseDSN != "" {
		t.Fatalf("unexpected database dsn: got %q, want empty", cfg.DatabaseDSN)
	}

	if cfg.MaxUploadSizeBytes != defaultMaxUploadSizeBytes {
		t.Fatalf(
			"unexpected max upload size: got %d, want %d",
			cfg.MaxUploadSizeBytes,
			defaultMaxUploadSizeBytes,
		)
	}
}

func TestLoadServer_FromEnv(t *testing.T) {
	t.Setenv(envServerAddress, "localhost:9090")
	t.Setenv(envLogLevel, "debug")
	t.Setenv(envDatabaseDSN, "postgres://user:pass@localhost:5432/db?sslmode=disable")
	t.Setenv(envMaxUploadSizeBytes, "12345")

	cfg := LoadServer()

	if cfg.Address != "localhost:9090" {
		t.Fatalf("unexpected address: got %q", cfg.Address)
	}

	if cfg.LogLevel != "debug" {
		t.Fatalf("unexpected log level: got %q", cfg.LogLevel)
	}

	if cfg.DatabaseDSN != "postgres://user:pass@localhost:5432/db?sslmode=disable" {
		t.Fatalf("unexpected database dsn: got %q", cfg.DatabaseDSN)
	}

	if cfg.MaxUploadSizeBytes != 12345 {
		t.Fatalf("unexpected max upload size: got %d, want %d", cfg.MaxUploadSizeBytes, 12345)
	}
}

func TestLoadServer_InvalidMaxUploadSizeUsesDefault(t *testing.T) {
	t.Setenv(envMaxUploadSizeBytes, "invalid")

	cfg := LoadServer()

	if cfg.MaxUploadSizeBytes != defaultMaxUploadSizeBytes {
		t.Fatalf(
			"unexpected max upload size: got %d, want %d",
			cfg.MaxUploadSizeBytes,
			defaultMaxUploadSizeBytes,
		)
	}
}

func TestLoadS3_FromEnv(t *testing.T) {
	t.Setenv(envS3Endpoint, "http://localhost:9000")
	t.Setenv(envS3Region, "eu-north-1")
	t.Setenv(envS3AccessKey, "access")
	t.Setenv(envS3SecretKey, "secret")
	t.Setenv(envS3Bucket, "avatars")
	t.Setenv(envS3UsePathStyle, "false")

	cfg := LoadS3()

	if cfg.Endpoint != "http://localhost:9000" {
		t.Fatalf("unexpected endpoint: got %q", cfg.Endpoint)
	}

	if cfg.Region != "eu-north-1" {
		t.Fatalf("unexpected region: got %q", cfg.Region)
	}

	if cfg.AccessKey != "access" {
		t.Fatalf("unexpected access key: got %q", cfg.AccessKey)
	}

	if cfg.SecretKey != "secret" {
		t.Fatalf("unexpected secret key: got %q", cfg.SecretKey)
	}

	if cfg.Bucket != "avatars" {
		t.Fatalf("unexpected bucket: got %q", cfg.Bucket)
	}

	if cfg.UsePathStyle {
		t.Fatal("expected use path style to be false")
	}
}

func TestLoadS3_InvalidBoolUsesDefault(t *testing.T) {
	t.Setenv(envS3UsePathStyle, "invalid")

	cfg := LoadS3()

	if cfg.UsePathStyle != defaultS3UsePathStyle {
		t.Fatalf("unexpected use path style: got %v, want %v", cfg.UsePathStyle, defaultS3UsePathStyle)
	}
}

func TestLoadRabbitMQ_Defaults(t *testing.T) {
	t.Setenv(envRabbitMQURL, "")
	t.Setenv(envRabbitMQExchange, "")
	t.Setenv(envRabbitMQUploadQueue, "")
	t.Setenv(envRabbitMQUploadRoutingKey, "")
	t.Setenv(envRabbitMQDeleteQueue, "")
	t.Setenv(envRabbitMQDeleteRoutingKey, "")

	cfg := LoadRabbitMQ()

	if cfg.URL != "" {
		t.Fatalf("unexpected url: got %q, want empty", cfg.URL)
	}

	if cfg.Exchange != defaultRabbitMQExchange {
		t.Fatalf("unexpected exchange: got %q", cfg.Exchange)
	}

	if cfg.UploadQueue != defaultRabbitMQUploadQueue {
		t.Fatalf("unexpected upload queue: got %q", cfg.UploadQueue)
	}

	if cfg.UploadRoutingKey != defaultRabbitMQUploadRoutingKey {
		t.Fatalf("unexpected upload routing key: got %q", cfg.UploadRoutingKey)
	}

	if cfg.DeleteQueue != defaultRabbitMQDeleteQueue {
		t.Fatalf("unexpected delete queue: got %q", cfg.DeleteQueue)
	}

	if cfg.DeleteRoutingKey != defaultRabbitMQDeleteRoutingKey {
		t.Fatalf("unexpected delete routing key: got %q", cfg.DeleteRoutingKey)
	}
}

func TestLoadWorker_FromEnv(t *testing.T) {
	t.Setenv(envLogLevel, "debug")
	t.Setenv(envDatabaseDSN, "postgres://user:pass@localhost:5432/db?sslmode=disable")

	cfg := LoadWorker()

	if cfg.LogLevel != "debug" {
		t.Fatalf("unexpected log level: got %q", cfg.LogLevel)
	}

	if cfg.DatabaseDSN != "postgres://user:pass@localhost:5432/db?sslmode=disable" {
		t.Fatalf("unexpected database dsn: got %q", cfg.DatabaseDSN)
	}
}
