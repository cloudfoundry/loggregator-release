package metron_test

import (
	"fmt"

	"github.com/cloudfoundry/dropsonde"
	"github.com/cloudfoundry/dropsonde/logs"
	"github.com/gorilla/websocket"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("communicating with doppler over UDP", func() {
	It("forwards messages", func() {
		etcdCleanup, etcdClientURL := setupEtcd()
		defer etcdCleanup()
		dopplerCleanup, dopplerOutgoingPort := setupDoppler(etcdClientURL)
		defer dopplerCleanup()
		metronCleanup, metronPort := setupMetron(etcdClientURL, "udp")
		defer metronCleanup()

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
			wsURL := fmt.Sprintf("ws://localhost:%d/apps/test-app-id/recentlogs", dopplerOutgoingPort)
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
