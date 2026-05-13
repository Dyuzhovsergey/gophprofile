package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRequireUserID_Success(t *testing.T) {
	var gotUserID string

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userID, ok := UserIDFromContext(r.Context())
		if !ok {
			t.Fatal("expected user id in context")
		}

		gotUserID = userID
		w.WriteHeader(http.StatusNoContent)
	})

	handler := RequireUserID(next)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-User-ID", " sergey ")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("unexpected status code: got %d, want %d", rec.Code, http.StatusNoContent)
	}

	if gotUserID != "sergey" {
		t.Fatalf("unexpected user id: got %q, want %q", gotUserID, "sergey")
	}
}

func TestRequireUserID_MissingHeader(t *testing.T) {
	nextCalled := false

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
	})

	handler := RequireUserID(next)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("unexpected status code: got %d, want %d", rec.Code, http.StatusBadRequest)
	}

	if nextCalled {
		t.Fatal("did not expect next handler to be called")
	}
}

func TestUserIDFromContext_Empty(t *testing.T) {
	userID, ok := UserIDFromContext(context.Background())
	if ok {
		t.Fatalf("expected no user id, got %q", userID)
	}
}
