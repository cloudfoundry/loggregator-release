package app_test

import (
	"net"

	"code.cloudfoundry.org/loggregator/healthendpoint"
	"code.cloudfoundry.org/loggregator/metricemitter"
	"code.cloudfoundry.org/loggregator/metron/app"
	"code.cloudfoundry.org/loggregator/plumbing"
	"code.cloudfoundry.org/loggregator/testservers"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/prometheus/client_golang/prometheus"
)

var _ = Describe("v2 App", func() {
	It("uses DopplerAddrWithAZ for AZ affinity", func() {
		spyLookup := newSpyLookup()
		gaugeMap := stubGaugeMap()

		promRegistry := prometheus.NewRegistry()
		he := healthendpoint.New(promRegistry, gaugeMap)
		clientCreds, err := plumbing.NewClientCredentials(
			testservers.Cert("metron.crt"),
			testservers.Cert("metron.key"),
			testservers.Cert("loggregator-ca.crt"),
			"doppler",
		)
		Expect(err).ToNot(HaveOccurred())

		serverCreds, err := plumbing.NewServerCredentials(
			testservers.Cert("doppler.crt"),
			testservers.Cert("doppler.key"),
			testservers.Cert("loggregator-ca.crt"),
		)
		Expect(err).ToNot(HaveOccurred())

		config := testservers.BuildMetronConfig("localhost", 1234)
		config.Zone = "something-bad"
		expectedHost, _, err := net.SplitHostPort(config.DopplerAddrWithAZ)
		Expect(err).ToNot(HaveOccurred())

		app := app.NewV2App(
			&config,
			he,
			clientCreds,
			serverCreds,
			spyMetricClient{},
			app.WithV2Lookup(spyLookup.lookup),
		)
		go app.Start()

		Eventually(spyLookup.calledWith(expectedHost)).Should(BeTrue())
	})
})

func (spyMetricClient) NewCounter(
	name string,
	opts ...metricemitter.MetricOption,
) *metricemitter.Counter {
	return metricemitter.NewCounter(name, "test-source-id")
}
