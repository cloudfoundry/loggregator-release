package reliability

import (
	"bytes"
	"encoding/json"
	"log"
	"net/http"
	"time"
)

type Runner interface {
	Run(t *Test)
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

	t := &Test{}
	if err := json.NewDecoder(r.Body).Decode(t); err != nil {
		log.Printf("failed to decode request body: %s", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	r.Body.Close()

	t.ID = time.Now().UnixNano()

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

// Test is used to decode the body from a request.
type Test struct {
	ID     int64  `json:"id"`
	Cycles uint64 `json:"cycles"`
	// How many writes an individual worker does. This value changes depending
	// on the worker.
	WriteCycles uint64   `json:"write_cycles"`
	Delay       Duration `json:"delay"`
	Timeout     Duration `json:"timeout"`
}

type Duration time.Duration

func (d *Duration) UnmarshalJSON(b []byte) error {
	val := bytes.Trim(b, `"`)
	dur, err := time.ParseDuration(string(val))

	if err != nil {
		return err
	}

	*d = Duration(dur)

	return nil
}

func (d *Duration) MarshalJSON() ([]byte, error) {
	return []byte("\"" + (time.Duration)(*d).String() + "\""), nil
}
