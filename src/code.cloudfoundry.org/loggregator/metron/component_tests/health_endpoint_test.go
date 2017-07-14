package component_test

import (
	"fmt"
	"io/ioutil"
	"net/http"

	"code.cloudfoundry.org/loggregator/testservers"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Metron Health Endpoint", func() {
	var (
		consumerServer *Server
		metronCleanup  func()
	)

	BeforeEach(func() {
		var err error
		consumerServer, err = NewServer()
		Expect(err).ToNot(HaveOccurred())

		var metronReady func()
		metronCleanup, _, metronReady = testservers.StartMetron(
			testservers.BuildMetronConfig("localhost", consumerServer.Port()),
		)
		metronReady()
	})

	AfterEach(func() {
		consumerServer.Stop()
		metronCleanup()
	})

	It("returns health metrics", func() {
		resp, err := http.Get(fmt.Sprintf("http://localhost:7629/health"))
		Expect(err).ToNot(HaveOccurred())
		defer resp.Body.Close()
		Expect(resp.StatusCode).To(Equal(http.StatusOK))
		body, err := ioutil.ReadAll(resp.Body)
		Expect(err).ToNot(HaveOccurred())

		Expect(body).To(ContainSubstring("dopplerConnections"))
		Expect(body).To(ContainSubstring("dopplerV1Streams"))
		Expect(body).To(ContainSubstring("dopplerV2Streams"))
	})
})
