package handlers

import (
	"encoding/json"
	"net/http"
)

// HealthResponse описывает ответ endpoint-а проверки работоспособности.
type HealthResponse struct {
	Status string `json:"status"`
}

// Health обрабатывает запрос проверки работоспособности сервиса.
func Health(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if err := json.NewEncoder(w).Encode(HealthResponse{Status: "ok"}); err != nil {
		http.Error(w, "failed to encode health response", http.StatusInternalServerError)
	}
}
