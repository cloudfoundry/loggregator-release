package reliability_test

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"tools/reliability"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("CreateTestHandler", func() {
	var (
		recorder *httptest.ResponseRecorder
		runner   *spyRunner
	)

	BeforeEach(func() {
		runner = &spyRunner{}
		h := reliability.NewCreateTestHandler(runner)
		recorder = httptest.NewRecorder()

		h.ServeHTTP(recorder, &http.Request{
			Method: "POST",
			Body: &requestBody{
				Reader: strings.NewReader(`{"cycles": 1000, "delay":"1s", "timeout":"60s"}`),
			},
		})
	})

	It("passes the test to a runner", func() {
		Expect(recorder.Code).To(Equal(http.StatusCreated))
		Eventually(runner.Count).Should(Equal(int64(1)))
	})

	It("responds with the created test", func() {
		body := recorder.Body.String()

		var test reliability.Test
		err := json.Unmarshal([]byte(body), &test)
		Expect(err).ToNot(HaveOccurred())
		Expect(test.ID).ToNot(Equal(int64(0)))

		Expect(body).To(MatchJSON(fmt.Sprintf(`{
			"id": %d,
			"cycles": 1000,
			"write_cycles": 0,
			"delay": "1s",
			"timeout": "1m0s"
		}`, test.ID)))
	})
})

type spyRunner struct {
	reliability.Runner
	runCallCount int64
}

func (s *spyRunner) Run(*reliability.Test) {
	atomic.AddInt64(&s.runCallCount, 1)
}

func (s *spyRunner) Count() int64 {
	return atomic.LoadInt64(&s.runCallCount)
}

type requestBody struct {
	io.Reader
	Closer io.Closer
}

func (r *requestBody) Close() error {
	return nil
}
