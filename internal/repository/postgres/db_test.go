package postgres

import (
	"context"
	"errors"
	"testing"
)

func TestNewPool_EmptyDSN(t *testing.T) {
	pool, err := NewPool(context.Background(), "")

	if !errors.Is(err, ErrEmptyDatabaseDSN) {
		t.Fatalf("expected ErrEmptyDatabaseDSN, got %v", err)
	}

	if pool != nil {
		t.Fatal("expected nil pool")
	}
}
