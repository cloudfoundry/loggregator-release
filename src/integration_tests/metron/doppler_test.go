package metron_test

import (
	"fmt"
	"integration_tests"

	"github.com/cloudfoundry/dropsonde"
	"github.com/cloudfoundry/dropsonde/logs"
	"github.com/gorilla/websocket"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("communicating with doppler", func() {
	It("forwards messages via gRPC", func() {
		etcdCleanup, etcdClientURL := integration_tests.SetupEtcd()
		defer etcdCleanup()
		dopplerCleanup, dopplerWSPort, dopplerGRPCPort := integration_tests.StartTestDoppler(
			integration_tests.BuildTestDopplerConfig(etcdClientURL, 0),
		)
		defer dopplerCleanup()

		metronCleanup, metronPort, metronReady := integration_tests.SetupMetron("localhost", dopplerGRPCPort, 0)
		defer metronCleanup()
		metronReady()

		err := dropsonde.Initialize(fmt.Sprintf("localhost:%d", metronPort), "test-origin")
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
