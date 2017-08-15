package api_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	sharedapi "tools/reliability/api"
	"tools/reliability/server/internal/api"

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
		h := api.NewCreateTestHandler(runner)
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

		var test sharedapi.Test
		err := json.Unmarshal([]byte(body), &test)
		Expect(err).ToNot(HaveOccurred())
		Expect(test.ID).ToNot(Equal(int64(0)))
		Expect(test.Cycles).To(Equal(uint64(1000)))
		Expect(test.Delay).To(Equal(
			sharedapi.Duration(
				int64(1000000000),
			),
		))
		Expect(test.Timeout).To(Equal(
			sharedapi.Duration(
				int64(60000000000),
			),
		))
		Expect(test.StartTime).ToNot(Equal(int64(0)))
	})
})

type spyRunner struct {
	api.Runner
	runCallCount int64
}

func (s *spyRunner) Run(*sharedapi.Test) {
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
