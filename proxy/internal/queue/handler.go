package queue

import (
	"encoding/json"
	"errors"
	"net/http"
)

type statusResponse struct {
	Position             int  `json:"position"`
	Released             bool `json:"released"`
	EstimatedWaitSeconds int  `json:"estimated_wait_seconds"`
}

type errorResponse struct {
	Error string `json:"error"`
}

// NewHandler returns the public /queue HTTP API (queue_id is the secret).
func NewHandler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /queue/status", handleStatus)
	return mux
}

func handleStatus(w http.ResponseWriter, r *http.Request) {
	tenant := r.URL.Query().Get("tenant")
	queueID := r.URL.Query().Get("queue_id")
	if tenant == "" || queueID == "" {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "tenant and queue_id are required"})
		return
	}

	pos, released, err := Position(tenant, queueID)
	if errors.Is(err, ErrUnknownTicket) {
		writeJSON(w, http.StatusGone, errorResponse{
			Error: "please retry, your queue slot expired",
		})
		return
	}
	if err != nil {
		writeJSON(w, http.StatusServiceUnavailable, errorResponse{Error: "queue temporarily unavailable"})
		return
	}

	wait := 0
	if !released && pos > 0 {
		batch := ReleaseBatchSize()
		if batch < 1 {
			batch = 1
		}
		wait = (pos / batch) * ReleaseIntervalSeconds()
	}

	writeJSON(w, http.StatusOK, statusResponse{
		Position:             pos,
		Released:             released,
		EstimatedWaitSeconds: wait,
	})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
