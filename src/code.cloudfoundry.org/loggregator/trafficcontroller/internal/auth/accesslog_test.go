package auth_test

import (
	"fmt"
	"math/rand"
	"net/http"
	"strconv"
	"time"

	"code.cloudfoundry.org/loggregator/trafficcontroller/internal/auth"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("AccessLog", func() {
	var (
		req       *http.Request
		timestamp time.Time
		al        *auth.AccessLog

		// request data
		method     string
		path       string
		url        string
		sourceHost string
		sourcePort string
		remoteAddr string
		dstHost    string
		dstPort    string

		forwardedFor string
		requestId    string
	)

	BeforeEach(func() {
		req = nil
		timestamp = time.Now()

		method = "GET"
		path = fmt.Sprintf("/some/path?with_query=params-%d", rand.Int())
		url = "http://example.com" + path
		sourceHost = fmt.Sprintf("10.0.1.%d", rand.Int()%256)
		sourcePort = strconv.Itoa(rand.Int()%65535 + 1)
		remoteAddr = sourceHost + ":" + sourcePort
		dstHost = fmt.Sprintf("10.1.2.%d", rand.Int()%256)
		dstPort = strconv.Itoa(rand.Int()%65535 + 1)

		forwardedFor = fmt.Sprintf("10.0.0.%d", rand.Int()%256)
		requestId = fmt.Sprintf("test-vcap-request-id-%d", rand.Int())
	})

	JustBeforeEach(func() {
		req = buildRequest(method, url, remoteAddr, requestId, forwardedFor)
		port, err := strconv.Atoi(dstPort)
		Expect(err).ToNot(HaveOccurred())
		al = auth.NewAccessLog(req, timestamp, dstHost, uint32(port))
	})

	Describe("String", func() {
		Context("with a GET request", func() {
			BeforeEach(func() {
				method = "GET"
			})

			It("returns a log with GET as the method", func() {
				expected := buildExpectedLog(
					timestamp,
					requestId,
					method,
					path,
					forwardedFor,
					"",
					dstHost,
					dstPort,
				)
				Expect(al.String()).To(Equal(expected))
			})
		})

		Context("with a POST request", func() {
			BeforeEach(func() {
				method = "POST"
			})

			It("returns a log with POST as the method", func() {
				expected := buildExpectedLog(
					timestamp,
					requestId,
					method,
					path,
					forwardedFor,
					"",
					dstHost,
					dstPort,
				)
				Expect(al.String()).To(Equal(expected))
			})
		})

		Context("with X-Forwarded-For not set", func() {
			BeforeEach(func() {
				forwardedFor = ""
			})

			It("uses remoteAddr", func() {
				expected := buildExpectedLog(
					timestamp,
					requestId,
					method,
					path,
					sourceHost,
					sourcePort,
					dstHost,
					dstPort,
				)
				Expect(al.String()).To(Equal(expected))
			})
		})

		Context("with X-Forwarded-For containing multiple values", func() {
			BeforeEach(func() {
				forwardedFor = "123.22.11.1, 6.3.4.5, 1.2.3.4"
			})

			It("uses remoteAddr", func() {
				expected := buildExpectedLog(
					timestamp,
					requestId,
					method,
					path,
					"123.22.11.1",
					"",
					dstHost,
					dstPort,
				)
				Expect(al.String()).To(Equal(expected))
			})
		})

		Context("with a request that has no query params", func() {
			BeforeEach(func() {
				path = "/some/path"
				url = "http://example.com" + path
			})

			It("writes log without question mark delimiter", func() {
				prefix := "CEF:0|cloud_foundry|loggregator_trafficcontroller|1.0|GET /some/path|"
				Expect(al.String()).To(HavePrefix(prefix))
			})
		})
	})
})
