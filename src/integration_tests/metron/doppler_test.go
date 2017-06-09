package metron_test

import (
	"fmt"

	"github.com/cloudfoundry/dropsonde"
	"github.com/cloudfoundry/dropsonde/logs"
	"github.com/gorilla/websocket"

	"code.cloudfoundry.org/loggregator/testservers"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("communicating with doppler", func() {
	XIt("forwards messages via gRPC", func() {
		etcdCleanup, etcdClientURL := testservers.StartTestEtcd()
		defer etcdCleanup()
		dopplerCleanup, dopplerWSPort, dopplerGRPCPort := testservers.StartDoppler(
			testservers.BuildDopplerConfig(etcdClientURL, 0, 0),
		)
		defer dopplerCleanup()

		metronCleanup, metronConfig, metronReady := testservers.StartMetron(
			testservers.BuildMetronConfig("localhost", dopplerGRPCPort),
		)
		defer metronCleanup()
		metronReady()

		err := dropsonde.Initialize(fmt.Sprintf("localhost:%d", metronConfig.IncomingUDPPort), "test-origin")
		Expect(err).NotTo(HaveOccurred())

		By("sending a message into metron")
		sent := make(chan struct{})
		go func() {
			defer close(sent)
			err := logs.SendAppLog("test-app-id", "An event happened!", "test-app-id", "0")
			Expect(err).NotTo(HaveOccurred())
		}()
		<-sent

		By("reading a message from doppler")
		Eventually(func() ([]byte, error) {
			wsURL := fmt.Sprintf("ws://localhost:%d/apps/test-app-id/recentlogs", dopplerWSPort)
			c, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
			if err != nil {
				return []byte{}, err
			}
			_, message, err := c.ReadMessage()
			if err != nil {
				return []byte{}, err
			}
			return message, err
		}).Should(ContainSubstring("An event happened!"))
	})
})
