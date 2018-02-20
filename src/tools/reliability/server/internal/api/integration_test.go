package api_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"time"
	"tools/reliability/server/internal/api"

	"github.com/posener/wstest"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

// TODO: assert against json being written to websocket connection
var _ = Describe("Server<->Worker communciation", func() {
	It("will record an error when there are no workers", func() {
		workerHandler := api.NewWorkerHandler()
		createTestHandler := api.NewCreateTestHandler(workerHandler, time.Millisecond)

		recorder := initiateTest(createTestHandler)
		Expect(recorder.Code).To(Equal(http.StatusInternalServerError))
	})

	It("creates a test when workers are available", func() {
		workerHandler := api.NewWorkerHandler()
		createTestHandler := api.NewCreateTestHandler(workerHandler, time.Second)
		cleanup := attachWorker(workerHandler)
		defer cleanup()

		recorder := initiateTest(createTestHandler)
		Expect(recorder.Code).To(Equal(http.StatusCreated))
	})
})

func initiateTest(createTestHandler http.Handler) *httptest.ResponseRecorder {
	body := strings.NewReader(`{"cycles": 1000, "delay":"1s", "timeout":"60s"}`)
	req, err := http.NewRequest("POST", "http://localhost/foo", body)
	Expect(err).ToNot(HaveOccurred())
	recorder := httptest.NewRecorder()
	createTestHandler.ServeHTTP(recorder, req)
	return recorder
}

func attachWorker(workerHandler http.Handler) func() {
	d := wstest.NewDialer(workerHandler)
	c, _, err := d.Dial("ws://localhost:8080/ws", nil)
	Expect(err).ToNot(HaveOccurred())
	return func() {
		c.Close()
	}
}
