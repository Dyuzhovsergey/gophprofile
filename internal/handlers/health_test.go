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
	handler := NewHealthHandler(
		fakeHealthPinger{},
		fakeHealthPinger{},
		fakeHealthPinger{},
	)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()

	handler.Handle(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected status code: got %d, want %d", rec.Code, http.StatusOK)
	}

	response := decodeHealthResponse(t, rec)

	if response.Status != "ok" {
		t.Fatalf("unexpected status: got %q, want %q", response.Status, "ok")
	}

	assertHealthDetail(t, response, "server", "ok")
	assertHealthDetail(t, response, "postgres", "ok")
	assertHealthDetail(t, response, "s3", "ok")
	assertHealthDetail(t, response, "rabbitmq", "ok")
}

func TestHealthHandler_Handle_PostgresError(t *testing.T) {
	handler := NewHealthHandler(
		fakeHealthPinger{err: errors.New("postgres is unavailable")},
		fakeHealthPinger{},
		fakeHealthPinger{},
	)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()

	handler.Handle(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf(
			"unexpected status code: got %d, want %d",
			rec.Code,
			http.StatusServiceUnavailable,
		)
	}

	response := decodeHealthResponse(t, rec)

	if response.Status != "degraded" {
		t.Fatalf("unexpected status: got %q, want %q", response.Status, "degraded")
	}

	assertHealthDetail(t, response, "postgres", "error")
	assertHealthDetail(t, response, "s3", "ok")
	assertHealthDetail(t, response, "rabbitmq", "ok")
}

func TestHealthHandler_Handle_S3Error(t *testing.T) {
	handler := NewHealthHandler(
		fakeHealthPinger{},
		fakeHealthPinger{err: errors.New("s3 is unavailable")},
		fakeHealthPinger{},
	)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()

	handler.Handle(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf(
			"unexpected status code: got %d, want %d",
			rec.Code,
			http.StatusServiceUnavailable,
		)
	}

	response := decodeHealthResponse(t, rec)

	if response.Status != "degraded" {
		t.Fatalf("unexpected status: got %q, want %q", response.Status, "degraded")
	}

	assertHealthDetail(t, response, "postgres", "ok")
	assertHealthDetail(t, response, "s3", "error")
	assertHealthDetail(t, response, "rabbitmq", "ok")
}

func TestHealthHandler_Handle_RabbitMQError(t *testing.T) {
	handler := NewHealthHandler(
		fakeHealthPinger{},
		fakeHealthPinger{},
		fakeHealthPinger{err: errors.New("rabbitmq is unavailable")},
	)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()

	handler.Handle(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf(
			"unexpected status code: got %d, want %d",
			rec.Code,
			http.StatusServiceUnavailable,
		)
	}

	response := decodeHealthResponse(t, rec)

	if response.Status != "degraded" {
		t.Fatalf("unexpected status: got %q, want %q", response.Status, "degraded")
	}

	assertHealthDetail(t, response, "postgres", "ok")
	assertHealthDetail(t, response, "s3", "ok")
	assertHealthDetail(t, response, "rabbitmq", "error")
}

func TestHealthHandler_Handle_NotConfigured(t *testing.T) {
	handler := NewHealthHandler(nil, nil, nil)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()

	handler.Handle(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf(
			"unexpected status code: got %d, want %d",
			rec.Code,
			http.StatusServiceUnavailable,
		)
	}

	response := decodeHealthResponse(t, rec)

	if response.Status != "degraded" {
		t.Fatalf("unexpected status: got %q, want %q", response.Status, "degraded")
	}

	assertHealthDetail(t, response, "server", "ok")
	assertHealthDetail(t, response, "postgres", "not_configured")
	assertHealthDetail(t, response, "s3", "not_configured")
	assertHealthDetail(t, response, "rabbitmq", "not_configured")
}

func decodeHealthResponse(t *testing.T, rec *httptest.ResponseRecorder) HealthResponse {
	t.Helper()

	var response HealthResponse
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	return response
}

func assertHealthDetail(
	t *testing.T,
	response HealthResponse,
	name string,
	want string,
) {
	t.Helper()

	got := response.Details[name]
	if got != want {
		t.Fatalf("unexpected %s status: got %q, want %q", name, got, want)
	}
}
