// +build linux

package trafficcontroller_test

import (
	"metron/networkreader"
	"metron/writers/eventunmarshaller"
	"sync/atomic"

	"github.com/cloudfoundry/loggregatorlib/loggertesthelper"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Monitor", func() {

	var (
		writer          *fakeWriter
		dropsondeReader *networkreader.NetworkReader
	)

	BeforeEach(func() {
		logger := loggertesthelper.Logger()
		writer = &fakeWriter{}
		var err error
		dropsondeUnmarshaller := eventunmarshaller.New(writer, logger)
		dropsondeReader, err = networkreader.New("127.0.0.1:37474", "dropsondeAgentListener", dropsondeUnmarshaller, logger)
		Expect(err).NotTo(HaveOccurred())
	})

	Context("LinuxFileDescriptor", func() {
		It("sends open file descriptor metrics", func() {
			defer dropsondeReader.Stop()
			go dropsondeReader.Start()

			Eventually(func() uint64 { return atomic.LoadUint64(&writer.openFileDescriptors) }, 3).Should(BeNumerically("~", 15, 3))
		})
	})
})
