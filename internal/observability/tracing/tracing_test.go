package tracing

import (
	"context"
	"errors"
	"testing"
)

func TestNewConfig_TrimsValues(t *testing.T) {
	cfg := NewConfig(" gophprofile-server ", " localhost:4318 ", true)

	if cfg.ServiceName != "gophprofile-server" {
		t.Fatalf("unexpected service name: got %q", cfg.ServiceName)
	}

	if cfg.ExporterEndpoint != "localhost:4318" {
		t.Fatalf("unexpected exporter endpoint: got %q", cfg.ExporterEndpoint)
	}

	if !cfg.Enabled {
		t.Fatal("expected tracing to be enabled")
	}
}

func TestInitProvider_Disabled(t *testing.T) {
	shutdown, err := InitProvider(context.Background(), Config{
		Enabled: false,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if shutdown == nil {
		t.Fatal("expected shutdown func")
	}

	if err := shutdown(context.Background()); err != nil {
		t.Fatalf("unexpected shutdown error: %v", err)
	}
}

func TestInitProvider_EnabledWithoutServiceName(t *testing.T) {
	shutdown, err := InitProvider(context.Background(), Config{
		Enabled:          true,
		ExporterEndpoint: "localhost:4318",
	})

	if !errors.Is(err, ErrEmptyServiceName) {
		t.Fatalf("expected ErrEmptyServiceName, got %v", err)
	}

	if shutdown != nil {
		t.Fatal("expected nil shutdown func")
	}
}

func TestInitProvider_EnabledWithoutExporterEndpoint(t *testing.T) {
	shutdown, err := InitProvider(context.Background(), Config{
		Enabled:     true,
		ServiceName: "gophprofile-server",
	})

	if !errors.Is(err, ErrEmptyExporterEndpoint) {
		t.Fatalf("expected ErrEmptyExporterEndpoint, got %v", err)
	}

	if shutdown != nil {
		t.Fatal("expected nil shutdown func")
	}
}
