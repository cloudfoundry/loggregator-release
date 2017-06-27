package trafficcontroller_test

import (
	"io/ioutil"
	"net/http"

	"code.cloudfoundry.org/loggregator/testservers"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("TrafficController Health Endpoint", func() {
	It("returns health metrics", func() {
		healthURL := testservers.InfoPollString(infoPath, "trafficcontroller", "health_url")
		resp, err := http.Get(healthURL)
		Expect(err).ToNot(HaveOccurred())
		defer resp.Body.Close()

		Expect(resp.StatusCode).To(Equal(http.StatusOK))
		body, err := ioutil.ReadAll(resp.Body)
		Expect(err).ToNot(HaveOccurred())
		Expect(body).To(ContainSubstring("firehoseStreamCount"))
		Expect(body).To(ContainSubstring("appStreamCount"))
	})
})
