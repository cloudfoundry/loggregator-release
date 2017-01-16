package doppler_test

import (
	"fmt"
	"metron/networkreader"
	"metron/writers/eventunmarshaller"
	"net"
	"sync/atomic"

	"github.com/apoydence/eachers/testhelpers"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Uptime Monitor", func() {

	var (
		writer          *fakeWriter
		dropsondeReader *networkreader.NetworkReader
	)

	BeforeEach(func() {
		writer = &fakeWriter{}

		mockBatcher := newMockEventBatcher()
		mockChainer := newMockBatchCounterChainer()
		testhelpers.AlwaysReturn(mockBatcher.BatchCounterOutput, mockChainer)
		testhelpers.AlwaysReturn(mockChainer.SetTagOutput, mockChainer)

		var err error
		dropsondeUnmarshaller := eventunmarshaller.New(writer, mockBatcher)
		dropsondeReader, err = networkreader.New(fmt.Sprintf(":%d", getUDPPort()), "dropsondeAgentListener", dropsondeUnmarshaller)
		Expect(err).NotTo(HaveOccurred())
	})

	Context("Uptime", func() {
		It("sends uptime metrics", func() {
			defer dropsondeReader.Stop()
			go dropsondeReader.Start()

			Eventually(func() uint64 { return atomic.LoadUint64(&writer.lastUptime) }, 3).Should(BeNumerically(">", 1))
		})
	})
})

func getUDPPort() int {
	addr, err := net.ResolveUDPAddr("udp", "localhost:0")
	if err != nil {
		panic(err)
	}

	c, err := net.ListenUDP("udp", addr)
	if err != nil {
		panic(err)
	}
	defer c.Close()
	return c.LocalAddr().(*net.UDPAddr).Port
}
