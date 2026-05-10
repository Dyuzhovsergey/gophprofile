package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCORS_AddsHeadersForCrossOriginRequest(t *testing.T) {
	nextCalled := false

	handler := CORS(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/avatars/avatar-id", nil)
	req.Header.Set("Origin", "http://localhost:3000")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected status code: got %d, want %d", rec.Code, http.StatusOK)
	}

	if !nextCalled {
		t.Fatal("expected next handler to be called")
	}

	assertCORSHeaders(t, rec)
}

func TestCORS_PreflightRequest(t *testing.T) {
	nextCalled := false

	handler := CORS(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodOptions, "/api/v1/avatars", nil)
	req.Header.Set("Origin", "http://localhost:3000")
	req.Header.Set("Access-Control-Request-Method", http.MethodPost)
	req.Header.Set("Access-Control-Request-Headers", "Content-Type, X-User-ID")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("unexpected status code: got %d, want %d", rec.Code, http.StatusNoContent)
	}

	if nextCalled {
		t.Fatal("did not expect next handler to be called for preflight request")
	}

	assertCORSHeaders(t, rec)
}

func TestCORS_SameOriginRequest(t *testing.T) {
	nextCalled := false

	handler := CORS(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected status code: got %d, want %d", rec.Code, http.StatusOK)
	}

	if !nextCalled {
		t.Fatal("expected next handler to be called")
	}

	if rec.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Fatal("did not expect CORS headers for request without Origin")
	}
}

func assertCORSHeaders(t *testing.T, rec *httptest.ResponseRecorder) {
	t.Helper()

	if rec.Header().Get("Access-Control-Allow-Origin") != corsAllowOrigin {
		t.Fatalf(
			"unexpected allow origin: got %q, want %q",
			rec.Header().Get("Access-Control-Allow-Origin"),
			corsAllowOrigin,
		)
	}

	if rec.Header().Get("Access-Control-Allow-Methods") != corsAllowMethods {
		t.Fatalf(
			"unexpected allow methods: got %q, want %q",
			rec.Header().Get("Access-Control-Allow-Methods"),
			corsAllowMethods,
		)
	}

	if rec.Header().Get("Access-Control-Allow-Headers") != corsAllowHeaders {
		t.Fatalf(
			"unexpected allow headers: got %q, want %q",
			rec.Header().Get("Access-Control-Allow-Headers"),
			corsAllowHeaders,
		)
	}

	if rec.Header().Get("Access-Control-Max-Age") != corsMaxAge {
		t.Fatalf(
			"unexpected max age: got %q, want %q",
			rec.Header().Get("Access-Control-Max-Age"),
			corsMaxAge,
		)
	}
}
