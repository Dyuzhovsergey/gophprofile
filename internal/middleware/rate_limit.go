package middleware

import (
	"encoding/json"
	"math"
	"net/http"
	"strconv"
	"sync"
	"time"
)

// rateLimitErrorResponse описывает ответ при превышении лимита.
type rateLimitErrorResponse struct {
	Error string `json:"error"`
}

// RateLimiter ограничивает частоту запросов с помощью Token Bucket.
//
// Ограничитель хранится в памяти одного server-процесса.
// Каждый Kubernetes Pod использует собственный экземпляр limiter-а.
type RateLimiter struct {
	enabled           bool
	requestsPerSecond float64
	burst             float64
	retryAfterSeconds string

	mu         sync.Mutex
	tokens     float64
	lastRefill time.Time
	now        func() time.Time
}

// NewRateLimiter создаёт ограничитель частоты HTTP-запросов.
func NewRateLimiter(
	enabled bool,
	requestsPerSecond float64,
	burst int,
) *RateLimiter {
	return newRateLimiter(
		enabled,
		requestsPerSecond,
		burst,
		time.Now,
	)
}

// newRateLimiter создаёт limiter с заданной функцией времени.
// Отдельный конструктор нужен для детерминированных тестов.
func newRateLimiter(
	enabled bool,
	requestsPerSecond float64,
	burst int,
	now func() time.Time,
) *RateLimiter {
	if requestsPerSecond <= 0 {
		requestsPerSecond = 1
	}

	if burst <= 0 {
		burst = 1
	}

	if now == nil {
		now = time.Now
	}

	retryAfterSeconds := int(
		math.Ceil(1 / requestsPerSecond),
	)
	if retryAfterSeconds < 1 {
		retryAfterSeconds = 1
	}

	currentTime := now()

	return &RateLimiter{
		enabled:           enabled,
		requestsPerSecond: requestsPerSecond,
		burst:             float64(burst),
		retryAfterSeconds: strconv.Itoa(retryAfterSeconds),
		tokens:            float64(burst),
		lastRefill:        currentTime,
		now:               now,
	}
}

// Allow сообщает, можно ли обработать следующий запрос.
func (l *RateLimiter) Allow() bool {
	if l == nil || !l.enabled {
		return true
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	currentTime := l.now()
	elapsedSeconds := currentTime.
		Sub(l.lastRefill).
		Seconds()

	if elapsedSeconds > 0 {
		l.tokens += elapsedSeconds * l.requestsPerSecond

		if l.tokens > l.burst {
			l.tokens = l.burst
		}

		l.lastRefill = currentTime
	}

	if l.tokens < 1 {
		return false
	}

	l.tokens--

	return true
}

// RetryAfter возвращает рекомендуемое время ожидания в секундах.
func (l *RateLimiter) RetryAfter() string {
	if l == nil {
		return "1"
	}

	return l.retryAfterSeconds
}

// RateLimit возвращает middleware ограничения частоты запросов.
func RateLimit(
	limiter *RateLimiter,
) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(
			w http.ResponseWriter,
			r *http.Request,
		) {
			if limiter == nil || limiter.Allow() {
				next.ServeHTTP(w, r)
				return
			}

			w.Header().Set(
				"Content-Type",
				"application/json",
			)
			w.Header().Set(
				"Retry-After",
				limiter.RetryAfter(),
			)
			w.WriteHeader(http.StatusTooManyRequests)

			_ = json.NewEncoder(w).Encode(
				rateLimitErrorResponse{
					Error: "rate limit exceeded",
				},
			)
		})
	}
}
