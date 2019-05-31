package router_test

import (
	"fmt"
	"io/ioutil"
	"net/http"

	"code.cloudfoundry.org/loggregator/testservers"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Doppler Health Endpoint", func() {
	It("returns health metrics", func() {
		dopplerCleanup, dopplerPorts := testservers.StartRouter(
			testservers.BuildRouterConfig(0, 0),
		)
		defer dopplerCleanup()

		resp, err := http.Get(fmt.Sprintf("http://127.0.0.1:%d/health", dopplerPorts.Health))
		Expect(err).ToNot(HaveOccurred())
		defer resp.Body.Close()
		Expect(resp.StatusCode).To(Equal(http.StatusOK))
		body, err := ioutil.ReadAll(resp.Body)
		Expect(err).ToNot(HaveOccurred())
		Expect(body).To(ContainSubstring("ingressStreamCount"))
		Expect(body).To(ContainSubstring("subscriptionCount"))
		Expect(body).To(ContainSubstring("recentLogCacheCount"))
	})
})
