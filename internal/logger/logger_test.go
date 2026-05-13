package logger

import "testing"

func TestInit_Success(t *testing.T) {
	log, err := Init("debug")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if log == nil {
		t.Fatal("expected logger")
	}

	_ = log.Sync()
}

func TestInit_InvalidLevel(t *testing.T) {
	log, err := Init("invalid")
	if err == nil {
		t.Fatal("expected error")
	}

	if log != nil {
		t.Fatal("expected nil logger")
	}
}
