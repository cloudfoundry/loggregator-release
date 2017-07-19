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

type CreateTestHandler struct {
	runner Runner
}

func NewCreateTestHandler(r Runner) *CreateTestHandler {
	return &CreateTestHandler{
		runner: r,
	}
}

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

type Test struct {
	Cycles  uint64   `json:"cycles"`
	Delay   Duration `json:"delay"`
	Timeout Duration `json:"timeout"`
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
