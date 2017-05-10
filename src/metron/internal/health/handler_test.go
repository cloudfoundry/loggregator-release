package health_test

import (
	"io/ioutil"
	"metron/internal/health"
	"net/http"
	"net/http/httptest"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Handler", func() {
	It("returns the health stats in JSON format", func() {
		registry := health.NewRegistry()
		value := registry.RegisterValue("test_count")
		value.Increment(10)

		recorder := httptest.NewRecorder()
		req, err := http.NewRequest(http.MethodGet, "http://localhost/something", nil)
		Expect(err).ToNot(HaveOccurred())

		handler := health.NewHandler(registry)
		handler.ServeHTTP(recorder, req)

		resp := recorder.Result()
		Expect(resp.StatusCode).To(Equal(http.StatusOK))
		defer resp.Body.Close()

		body, err := ioutil.ReadAll(resp.Body)
		Expect(err).ToNot(HaveOccurred())

		Expect(string(body)).To(MatchJSON(`{"test_count":10}`))
	})
})
