package api

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"
	sharedapi "tools/reliability/api"
)

// Runner tells the children to run tests.
type Runner interface {
	Run(t *sharedapi.Test) (int, error)
}

// CreateTestHandler handles HTTP requests (POST only) to initiate tests
// for the worker cluster. This should be called from a CI.
type CreateTestHandler struct {
	runner        Runner
	runnerTimeout time.Duration
}

// NewCreateTestHandler builds a new CreateTestHandler.
func NewCreateTestHandler(r Runner, runnerTimeout time.Duration) *CreateTestHandler {
	return &CreateTestHandler{
		runner:        r,
		runnerTimeout: runnerTimeout,
	}
}

// ServeHTTP implements http.Handler.
func (h *CreateTestHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	t, err := buildTest(r.Body)
	if err != nil {
		log.Printf("failed to decode request body: %s", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	if !valid(t) {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	err = h.attemptRun(t)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(w, err)
		return
	}

	resp, err := json.Marshal(t)
	if err != nil {
		log.Printf("failed to encode response: %s", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
	_, err = w.Write(resp)
	if err != nil {
		log.Printf("failed to write response: %s", err)
	}
}

func (h *CreateTestHandler) attemptRun(t *sharedapi.Test) error {
	timeout := time.After(h.runnerTimeout)
	var err error
	for {
		_, err = h.runner.Run(t)
		if err == nil {
			return nil
		}

		select {
		case <-timeout:
			return err
		default:
		}
	}
}

func buildTest(src io.ReadCloser) (*sharedapi.Test, error) {
	t := &sharedapi.Test{}
	err := json.NewDecoder(src).Decode(t)
	if err != nil {
		return nil, err
	}
	_ = src.Close()

	t.ID = time.Now().UnixNano()
	// ensure the test is sent to workers with a start time
	t.StartTime = time.Now()
	return t, nil
}

func valid(t *sharedapi.Test) bool {
	if t.Cycles == 0 {
		return false
	}
	if t.Timeout == 0 {
		return false
	}
	return true
}
