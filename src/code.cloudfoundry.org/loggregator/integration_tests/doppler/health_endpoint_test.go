package doppler_test

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
		etcdCleanup, etcdClientURL := testservers.StartTestEtcd()
		defer etcdCleanup()
		dopplerCleanup, dopplerPorts := testservers.StartDoppler(
			testservers.BuildDopplerConfig(etcdClientURL, 0, 0),
		)
		defer dopplerCleanup()

		resp, err := http.Get(fmt.Sprintf("http://localhost:%d/health", dopplerPorts.Health))
		Expect(err).ToNot(HaveOccurred())
		defer resp.Body.Close()
		Expect(resp.StatusCode).To(Equal(http.StatusOK))
		body, err := ioutil.ReadAll(resp.Body)
		Expect(err).ToNot(HaveOccurred())
		Expect(body).To(ContainSubstring("ingressStreamCount"))
		Expect(body).To(ContainSubstring("subscriptionCount"))
		Expect(body).To(ContainSubstring("recentLogCacheCount"))
		Expect(body).To(ContainSubstring("containerMetricCacheCount"))
	})
})
