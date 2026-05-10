package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"time"
)

const healthCheckTimeout = 2 * time.Second

// HealthPinger описывает компонент, который умеет проверять своё состояние.
type HealthPinger interface {
	Ping(ctx context.Context) error
}

// HealthHandler обрабатывает запросы проверки работоспособности сервиса.
type HealthHandler struct {
	postgres HealthPinger
	s3       HealthPinger
	rabbitMQ HealthPinger
}

// NewHealthHandler создаёт обработчик проверки работоспособности.
func NewHealthHandler(
	postgres HealthPinger,
	s3 HealthPinger,
	rabbitMQ HealthPinger,
) *HealthHandler {
	return &HealthHandler{
		postgres: postgres,
		s3:       s3,
		rabbitMQ: rabbitMQ,
	}
}

// HealthResponse описывает ответ endpoint-а проверки работоспособности.
type HealthResponse struct {
	Status  string            `json:"status"`
	Details map[string]string `json:"details"`
}

// Handle обрабатывает запрос проверки работоспособности сервиса.
func (h *HealthHandler) Handle(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	details := map[string]string{
		"server": "ok",
	}

	ctx, cancel := context.WithTimeout(r.Context(), healthCheckTimeout)
	defer cancel()

	allOK := true

	if !checkHealthComponent(ctx, details, "postgres", h.postgres) {
		allOK = false
	}

	if !checkHealthComponent(ctx, details, "s3", h.s3) {
		allOK = false
	}

	if !checkHealthComponent(ctx, details, "rabbitmq", h.rabbitMQ) {
		allOK = false
	}

	statusCode := http.StatusOK
	status := "ok"

	if !allOK {
		statusCode = http.StatusServiceUnavailable
		status = "degraded"
	}

	w.WriteHeader(statusCode)

	if err := json.NewEncoder(w).Encode(HealthResponse{
		Status:  status,
		Details: details,
	}); err != nil {
		http.Error(w, "failed to encode health response", http.StatusInternalServerError)
	}
}

// checkHealthComponent проверяет один компонент и записывает его статус в details.
func checkHealthComponent(
	ctx context.Context,
	details map[string]string,
	name string,
	pinger HealthPinger,
) bool {
	if pinger == nil {
		details[name] = "not_configured"
		return false
	}

	if err := pinger.Ping(ctx); err != nil {
		details[name] = "error"
		return false
	}

	details[name] = "ok"

	return true
}
