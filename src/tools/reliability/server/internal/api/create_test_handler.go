package api

import (
	"encoding/json"
	"log"
	"net/http"
	"time"
	sharedapi "tools/reliability/api"
)

// Runner tells the children to run tests.
type Runner interface {
	Run(t *sharedapi.Test)
}

// CreateTestHandler handles HTTP requests (POST only) to initiate tests
// for the worker cluster. This should be called from a CI.
type CreateTestHandler struct {
	runner Runner
}

// NewCreateTestHandler builds a new CreateTestHandler.
func NewCreateTestHandler(r Runner) *CreateTestHandler {
	return &CreateTestHandler{
		runner: r,
	}
}

// ServeHTTP implements http.Handler.
func (h *CreateTestHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	t := &sharedapi.Test{}
	if err := json.NewDecoder(r.Body).Decode(t); err != nil {
		log.Printf("failed to decode request body: %s", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	r.Body.Close()

	t.ID = time.Now().UnixNano()

	//Ensure the test is sent to Workers with a start time
	t.StartTime = time.Now().UnixNano()

	go h.runner.Run(t)

	resp, err := json.Marshal(t)
	if err != nil {
		log.Printf("failed to encode response: %s", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
	w.Write(resp)
}
