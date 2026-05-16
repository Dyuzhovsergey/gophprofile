package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Dyuzhovsergey/gophprofile/internal/logger"
)

func TestRecover_Panic(t *testing.T) {
	handler := Recover(logger.NewNop())(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("boom")
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf(
			"unexpected status code: got %d, want %d",
			rec.Code,
			http.StatusInternalServerError,
		)
	}
}

func TestRecover_NoPanic(t *testing.T) {
	handler := Recover(logger.NewNop())(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("unexpected status code: got %d, want %d", rec.Code, http.StatusNoContent)
	}
}
