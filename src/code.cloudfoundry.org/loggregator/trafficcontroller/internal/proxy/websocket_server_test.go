package proxy_test

import (
	"net/http"
	"net/http/httptest"
	"time"

	"code.cloudfoundry.org/loggregator/metricemitter/testhelper"
	. "code.cloudfoundry.org/loggregator/trafficcontroller/internal/proxy"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("WebsocketServer", func() {
	It("increments a slow consumer counter", func() {
		mc := testhelper.NewMetricClient()
		em := mc.NewCounter("egress")
		s := NewWebSocketServer(time.Millisecond, mc)

		req, _ := http.NewRequest("GET", "/some", nil)

		s.ServeWS(httptest.NewRecorder(), req, func() ([]byte, error) {
			return []byte("hello"), nil
		}, em)

		Eventually(func() uint64 {
			return mc.GetDelta("doppler_proxy.slow_consumer")
		}).ShouldNot(BeZero())
	})
})
