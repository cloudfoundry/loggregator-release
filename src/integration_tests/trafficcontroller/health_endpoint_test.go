package trafficcontroller_test

import (
	"crypto/tls"
	"fmt"
	"io/ioutil"
	"net/http"

	"code.cloudfoundry.org/loggregator/testservers"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("TrafficController Health Endpoint", func() {
	It("returns health metrics", func() {
		httpClient := &http.Client{
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{
					InsecureSkipVerify: true,
				},
			},
		}
		cfg := testservers.BuildTrafficControllerConf("", 37474, "")

		var tcPorts testservers.TrafficControllerPorts
		tcCleanupFunc, tcPorts = testservers.StartTrafficController(cfg)

		// wait for TC
		trafficControllerDropsondeEndpoint := fmt.Sprintf(
			"https://%s:%d",
			localIPAddress,
			tcPorts.WS,
		)
		Eventually(func() error {
			resp, err := httpClient.Get(trafficControllerDropsondeEndpoint)
			if err == nil {
				resp.Body.Close()
			}
			return err
		}, 10).Should(Succeed())

		resp, err := http.Get(fmt.Sprintf("http://%s:%d/health", localIPAddress, tcPorts.Health))
		Expect(err).ToNot(HaveOccurred())
		defer resp.Body.Close()
		Expect(resp.StatusCode).To(Equal(http.StatusOK))
		body, err := ioutil.ReadAll(resp.Body)
		Expect(err).ToNot(HaveOccurred())
		Expect(body).To(ContainSubstring("firehoseStreamCount"))
		Expect(body).To(ContainSubstring("appStreamCount"))
		Expect(body).To(ContainSubstring("slowConsumerCount"))
	})
})
