package s3

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestClient_Upload_EmptyKey(t *testing.T) {
	client := &Client{}

	err := client.Upload(context.Background(), "   ", strings.NewReader("data"), "text/plain")
	if !errors.Is(err, ErrEmptyObjectKey) {
		t.Fatalf("expected ErrEmptyObjectKey, got %v", err)
	}
}

func TestClient_Download_EmptyKey(t *testing.T) {
	client := &Client{}

	data, contentType, err := client.Download(context.Background(), "   ")
	if !errors.Is(err, ErrEmptyObjectKey) {
		t.Fatalf("expected ErrEmptyObjectKey, got %v", err)
	}

	if data != nil {
		t.Fatal("expected nil data")
	}

	if contentType != "" {
		t.Fatalf("expected empty content type, got %q", contentType)
	}
}

func TestClient_Delete_EmptyKey(t *testing.T) {
	client := &Client{}

	err := client.Delete(context.Background(), "   ")
	if !errors.Is(err, ErrEmptyObjectKey) {
		t.Fatalf("expected ErrEmptyObjectKey, got %v", err)
	}
}

func TestClient_Exists_EmptyKey(t *testing.T) {
	client := &Client{}

	exists, err := client.Exists(context.Background(), "   ")
	if !errors.Is(err, ErrEmptyObjectKey) {
		t.Fatalf("expected ErrEmptyObjectKey, got %v", err)
	}

	if exists {
		t.Fatal("expected exists=false")
	}
}
