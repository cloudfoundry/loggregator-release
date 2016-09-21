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

var _ = Describe("communicating with doppler over TCP", func() {
	It("forwards messages", func() {
		etcdCleanup, etcdClientURL := integration_tests.SetupEtcd()
		defer etcdCleanup()
		metronCleanup, metronPort, metronReady := integration_tests.SetupMetron(etcdClientURL, "tcp")
		defer metronCleanup()
		dopplerCleanup, dopplerWSPort, _ := integration_tests.SetupDoppler(etcdClientURL, metronPort)
		defer dopplerCleanup()
		metronReady()

		err := dropsonde.Initialize(fmt.Sprintf("localhost:%d", metronPort), "test-origin")
		Expect(err).NotTo(HaveOccurred())

		By("sending a message into metron")
		sent := make(chan struct{})
		go func() {
			defer close(sent)
			err = logs.SendAppLog("test-app-id", "An event happened!", "test-app-id", "0")
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
