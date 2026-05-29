package logger

import "testing"

func TestInit_Success(t *testing.T) {
	log, err := Init("debug", "gophprofile-test", "test")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if log == nil {
		t.Fatal("expected logger")
	}
}

func TestInit_InvalidLevel(t *testing.T) {
	log, err := Init("invalid", "gophprofile-test", "test")
	if err == nil {
		t.Fatal("expected error")
	}

	if log != nil {
		t.Fatal("expected nil logger")
	}
}

func TestNewNop(t *testing.T) {
	log := NewNop()
	if log == nil {
		t.Fatal("expected logger")
	}
}
