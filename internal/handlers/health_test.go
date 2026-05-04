package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

type fakeHealthPinger struct {
	err error
}

func (p fakeHealthPinger) Ping(ctx context.Context) error {
	return p.err
}

func TestHealthHandler_Handle_OK(t *testing.T) {
	handler := NewHealthHandler(fakeHealthPinger{})

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()

	handler.Handle(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected status code: got %d, want %d", rec.Code, http.StatusOK)
	}

	var response HealthResponse
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if response.Status != "ok" {
		t.Fatalf("unexpected status: got %q, want %q", response.Status, "ok")
	}

	if response.Details["server"] != "ok" {
		t.Fatalf("unexpected server status: got %q, want %q", response.Details["server"], "ok")
	}

	if response.Details["postgres"] != "ok" {
		t.Fatalf("unexpected postgres status: got %q, want %q", response.Details["postgres"], "ok")
	}
}

func TestHealthHandler_Handle_PostgresError(t *testing.T) {
	handler := NewHealthHandler(fakeHealthPinger{
		err: errors.New("postgres is unavailable"),
	})

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()

	handler.Handle(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("unexpected status code: got %d, want %d", rec.Code, http.StatusServiceUnavailable)
	}

	var response HealthResponse
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if response.Status != "degraded" {
		t.Fatalf("unexpected status: got %q, want %q", response.Status, "degraded")
	}

	if response.Details["postgres"] != "error" {
		t.Fatalf("unexpected postgres status: got %q, want %q", response.Details["postgres"], "error")
	}
}

func TestHealthHandler_Handle_PostgresNotConfigured(t *testing.T) {
	handler := NewHealthHandler(nil)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()

	handler.Handle(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("unexpected status code: got %d, want %d", rec.Code, http.StatusServiceUnavailable)
	}

	var response HealthResponse
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if response.Status != "degraded" {
		t.Fatalf("unexpected status: got %q, want %q", response.Status, "degraded")
	}

	if response.Details["postgres"] != "not_configured" {
		t.Fatalf("unexpected postgres status: got %q, want %q", response.Details["postgres"], "not_configured")
	}
}
