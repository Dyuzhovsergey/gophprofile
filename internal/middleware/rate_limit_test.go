package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestRateLimiter_AllowsRequestsWithinBurst(t *testing.T) {
	currentTime := time.Unix(1_700_000_000, 0)

	limiter := newRateLimiter(
		true,
		2,
		2,
		func() time.Time {
			return currentTime
		},
	)

	if !limiter.Allow() {
		t.Fatal("expected first request to be allowed")
	}

	if !limiter.Allow() {
		t.Fatal("expected second request to be allowed")
	}

	if limiter.Allow() {
		t.Fatal("expected third request to be rejected")
	}
}

func TestRateLimiter_RefillsTokens(t *testing.T) {
	currentTime := time.Unix(1_700_000_000, 0)

	limiter := newRateLimiter(
		true,
		2,
		1,
		func() time.Time {
			return currentTime
		},
	)

	if !limiter.Allow() {
		t.Fatal("expected first request to be allowed")
	}

	if limiter.Allow() {
		t.Fatal("expected second request to be rejected")
	}

	// При скорости 2 запроса в секунду один токен
	// появляется через 500 миллисекунд.
	currentTime = currentTime.Add(500 * time.Millisecond)

	if !limiter.Allow() {
		t.Fatal("expected request to be allowed after refill")
	}
}

func TestRateLimiter_Disabled(t *testing.T) {
	limiter := newRateLimiter(
		false,
		1,
		1,
		time.Now,
	)

	for requestNumber := 1; requestNumber <= 100; requestNumber++ {
		if !limiter.Allow() {
			t.Fatalf(
				"request %d was unexpectedly rejected",
				requestNumber,
			)
		}
	}
}

func TestRateLimit_ReturnsTooManyRequests(t *testing.T) {
	currentTime := time.Unix(1_700_000_000, 0)

	limiter := newRateLimiter(
		true,
		1,
		1,
		func() time.Time {
			return currentTime
		},
	)

	nextCalled := false

	handler := RateLimit(limiter)(
		http.HandlerFunc(func(
			w http.ResponseWriter,
			_ *http.Request,
		) {
			nextCalled = true
			w.WriteHeader(http.StatusOK)
		}),
	)

	firstResponse := httptest.NewRecorder()

	handler.ServeHTTP(
		firstResponse,
		httptest.NewRequest(http.MethodGet, "/", nil),
	)

	if firstResponse.Code != http.StatusOK {
		t.Fatalf(
			"unexpected first status: got %d, want %d",
			firstResponse.Code,
			http.StatusOK,
		)
	}

	if !nextCalled {
		t.Fatal("expected next handler to be called")
	}

	nextCalled = false

	secondResponse := httptest.NewRecorder()

	handler.ServeHTTP(
		secondResponse,
		httptest.NewRequest(http.MethodGet, "/", nil),
	)

	if secondResponse.Code != http.StatusTooManyRequests {
		t.Fatalf(
			"unexpected second status: got %d, want %d",
			secondResponse.Code,
			http.StatusTooManyRequests,
		)
	}

	if nextCalled {
		t.Fatal("did not expect next handler to be called")
	}

	if secondResponse.Header().Get("Retry-After") != "1" {
		t.Fatalf(
			"unexpected Retry-After header: %q",
			secondResponse.Header().Get("Retry-After"),
		)
	}

	if contentType := secondResponse.Header().Get("Content-Type"); contentType != "application/json" {
		t.Fatalf(
			"unexpected Content-Type: got %q, want %q",
			contentType,
			"application/json",
		)
	}

	if !strings.Contains(
		secondResponse.Body.String(),
		"rate limit exceeded",
	) {
		t.Fatalf(
			"unexpected response body: %q",
			secondResponse.Body.String(),
		)
	}
}
