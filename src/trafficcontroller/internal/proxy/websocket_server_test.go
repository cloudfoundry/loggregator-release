package proxy_test

import (
	"net/http"
	"net/http/httptest"
	"regexp"
	"time"

	"code.cloudfoundry.org/loggregator-release/src/metricemitter/testhelper"
	. "code.cloudfoundry.org/loggregator-release/src/trafficcontroller/internal/proxy"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("WebsocketServer", func() {
	Describe("Slow Consumer", func() {
		var (
			metricClient *testhelper.SpyMetricClient
		)

		BeforeEach(func() {
			metricClient = testhelper.NewMetricClient()
			em := metricClient.NewCounter("egress")
			s := NewWebSocketServer(time.Millisecond, metricClient)

			req, _ := http.NewRequest("GET", "/some", nil)
			req.RemoteAddr = "some-address"
			req.Header["X-Forwarded-For"] = []string{"192.0.0.1", "192.0.0.2"}

			s.ServeWS(httptest.NewRecorder(), req, func() ([]byte, error) {
				return []byte("hello"), nil
			}, em)
		})

		It("increments a counter", func() {
			Eventually(func() uint64 {
				return metricClient.GetDelta("doppler_proxy.slow_consumer")
			}).ShouldNot(BeZero())
		})

		It("emits an event", func() {
			expectedBody := sanitizeWhitespace(`
Remote Address: some-address
X-Forwarded-For: 192.0.0.1, 192.0.0.2
Path: /some

When Loggregator detects a slow connection, that connection is disconnected to
prevent back pressure on the system. This may be due to improperly scaled
nozzles, or slow user connections to Loggregator`)

			Eventually(func() string {
				return sanitizeWhitespace(metricClient.GetEvent("Traffic Controller has disconnected slow consumer"))
			}).Should(Equal(expectedBody))
		})
	})
})

func sanitizeWhitespace(s string) string {
	return regexp.MustCompile(`\s`).ReplaceAllString(s, "")
}
