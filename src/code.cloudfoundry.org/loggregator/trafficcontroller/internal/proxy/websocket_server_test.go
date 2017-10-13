package proxy_test

import (
	"net/http"
	"net/http/httptest"
	"regexp"
	"time"

	"code.cloudfoundry.org/loggregator/metricemitter/testhelper"
	. "code.cloudfoundry.org/loggregator/trafficcontroller/internal/proxy"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("WebsocketServer", func() {
	Describe("Slow Consumer", func() {
		var (
			metricClient *testhelper.SpyMetricClient
			mockHealth   *mockHealth
		)

		BeforeEach(func() {
			metricClient = testhelper.NewMetricClient()
			mockHealth = newMockHealth()
			em := metricClient.NewCounter("egress")
			s := NewWebSocketServer(time.Millisecond, metricClient, mockHealth)

			req, _ := http.NewRequest("GET", "/some", nil)

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
When Loggregator detects a slow connection, that connection is disconnected to
prevent back pressure on the system. This may be due to improperly scaled
nozzles, or slow user connections to Loggregator`)

			Eventually(func() string {
				return sanitizeWhitespace(metricClient.GetEvent("Traffic Controller has disconnected slow consumer"))
			}).Should(Equal(expectedBody))
		})

		It("increments a health counter", func() {
			Eventually(mockHealth.IncInput.Name).Should(Receive(Equal("slowConsumerCount")))
		})
	})
})

func sanitizeWhitespace(s string) string {
	return regexp.MustCompile(`\s`).ReplaceAllString(s, "")
}
