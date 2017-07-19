package reliability_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"time"
	"tools/reliability"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("CreateTestHandler", func() {
	It("passes the test to a runner", func() {
		runner := &spyRunner{}
		h := reliability.NewCreateTestHandler(runner)
		recorder := httptest.NewRecorder()

		h.ServeHTTP(recorder, &http.Request{
			Method: "POST",
			Body: &requestBody{
				Reader: strings.NewReader(`{"cycles": 1000, "delay":"1s", "timeout":"60s"}`),
			},
		})

		Expect(recorder.Code).To(Equal(http.StatusCreated))
	})

	It("decodes JSON", func() {
		expected := &reliability.Test{
			Cycles:  1000,
			Delay:   reliability.Duration(1 * time.Second),
			Timeout: reliability.Duration(60 * time.Second),
		}
		b, err := json.Marshal(expected)
		Expect(err).NotTo(HaveOccurred())
		Expect(b).To(MatchJSON([]byte(`{"cycles": 1000, "delay": 1000000000, "timeout": 60000000000}`)))

		t := &reliability.Test{}
		r := strings.NewReader(`{"cycles": 1000, "delay": "1s", "timeout": "1m"}`)
		err = json.NewDecoder(r).Decode(t)
		Expect(err).NotTo(HaveOccurred())

		Expect(t).To(Equal(expected))
	})
})

type spyRunner struct {
	reliability.Runner
}

func (*spyRunner) Run(*reliability.Test) {}

type requestBody struct {
	io.Reader
	Closer io.Closer
}

func (r *requestBody) Close() error {
	return nil
}
