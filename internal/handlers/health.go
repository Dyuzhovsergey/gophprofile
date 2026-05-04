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
	db HealthPinger
}

// NewHealthHandler создаёт обработчик проверки работоспособности.
func NewHealthHandler(db HealthPinger) *HealthHandler {
	return &HealthHandler{
		db: db,
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

	statusCode := http.StatusOK
	status := "ok"

	if h.db == nil {
		details["postgres"] = "not_configured"
		status = "degraded"
		statusCode = http.StatusServiceUnavailable
	} else {
		ctx, cancel := context.WithTimeout(r.Context(), healthCheckTimeout)
		defer cancel()

		if err := h.db.Ping(ctx); err != nil {
			details["postgres"] = "error"
			status = "degraded"
			statusCode = http.StatusServiceUnavailable
		} else {
			details["postgres"] = "ok"
		}
	}

	w.WriteHeader(statusCode)

	if err := json.NewEncoder(w).Encode(HealthResponse{
		Status:  status,
		Details: details,
	}); err != nil {
		http.Error(w, "failed to encode health response", http.StatusInternalServerError)
	}
}
