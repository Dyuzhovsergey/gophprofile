package s3

import (
	"context"
	"errors"
	"testing"

	"github.com/Dyuzhovsergey/gophprofile/internal/config"
)

func TestNewClient_EmptyEndpoint(t *testing.T) {
	client, err := NewClient(context.Background(), config.S3Config{
		Bucket: "avatars",
	})

	if !errors.Is(err, ErrEmptyEndpoint) {
		t.Fatalf("expected ErrEmptyEndpoint, got %v", err)
	}

	if client != nil {
		t.Fatal("expected nil client")
	}
}

func TestNewClient_EmptyBucket(t *testing.T) {
	client, err := NewClient(context.Background(), config.S3Config{
		Endpoint: "http://localhost:9000",
	})

	if !errors.Is(err, ErrEmptyBucket) {
		t.Fatalf("expected ErrEmptyBucket, got %v", err)
	}

	if client != nil {
		t.Fatal("expected nil client")
	}
}
