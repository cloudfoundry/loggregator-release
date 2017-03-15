package doppler_test

import (
	"net"
	"time"

	"github.com/cloudfoundry/dropsonde/factories"
	"github.com/cloudfoundry/sonde-go/events"
	"github.com/nu7hatch/gouuid"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Container Metrics", func() {
	var receivedChan chan []byte
	var inputConnection net.Conn
	var appID string

	itDoesContainerMetrics := func(send func(events.Event, net.Conn) error) {
		It("returns container metrics for an app", func() {
			containerMetric := factories.NewContainerMetric(appID, 0, 1, 2, 3)

			send(containerMetric, inputConnection)

			time.Sleep(5 * time.Second)

			receivedChan = make(chan []byte)
			ws, _ := AddWSSink(receivedChan, "4567", "/apps/"+appID+"/containermetrics")
			defer ws.Close()

			var receivedMessageBytes []byte
			Eventually(receivedChan).Should(Receive(&receivedMessageBytes))

			receivedEnvelope := UnmarshalMessage(receivedMessageBytes)

			Expect(receivedEnvelope.GetEventType()).To(Equal(events.Envelope_ContainerMetric))
			receivedMetric := receivedEnvelope.GetContainerMetric()
			Expect(receivedMetric).To(Equal(containerMetric))
		})

		It("does not receive metrics for different appIds", func() {
			send(factories.NewContainerMetric(appID+"other", 0, 1, 2, 3), inputConnection)

			goodMetric := factories.NewContainerMetric(appID, 0, 100, 2, 3)
			send(goodMetric, inputConnection)

			send(factories.NewContainerMetric(appID+"other", 1, 1, 2, 3), inputConnection)

			time.Sleep(5 * time.Second)

			receivedChan = make(chan []byte)
			ws, _ := AddWSSink(receivedChan, "4567", "/apps/"+appID+"/containermetrics")
			defer ws.Close()

			var receivedMessageBytes []byte
			Eventually(receivedChan).Should(Receive(&receivedMessageBytes))
			Eventually(receivedChan).Should(BeClosed())

			receivedEnvelope := UnmarshalMessage(receivedMessageBytes)

			Expect(receivedEnvelope.GetContainerMetric().GetApplicationId()).To(Equal(appID))
			Expect(receivedEnvelope.GetContainerMetric()).To(Equal(goodMetric))
		})

		It("returns metrics for all instances of the app", func() {
			send(factories.NewContainerMetric(appID, 0, 1, 2, 3), inputConnection)
			send(factories.NewContainerMetric(appID, 1, 1, 2, 3), inputConnection)

			time.Sleep(5 * time.Second)

			receivedChan = make(chan []byte)
			ws, _ := AddWSSink(receivedChan, "4567", "/apps/"+appID+"/containermetrics")
			defer ws.Close()

			var firstReceivedMessageBytes []byte
			var secondReceivedMessageBytes []byte

			Eventually(receivedChan).Should(Receive(&firstReceivedMessageBytes))
			Eventually(receivedChan).Should(Receive(&secondReceivedMessageBytes))

			firstEnvelope := UnmarshalMessage(firstReceivedMessageBytes)
			secondEnvelope := UnmarshalMessage(secondReceivedMessageBytes)

			Expect(firstEnvelope.GetContainerMetric().GetApplicationId()).To(Equal(appID))
			Expect(secondEnvelope.GetContainerMetric().GetApplicationId()).To(Equal(appID))
			Expect(firstEnvelope.GetContainerMetric()).NotTo(Equal(secondEnvelope.GetContainerMetric()))
		})

		It("returns only the latest container metric", func() {
			send(factories.NewContainerMetric(appID, 0, 10, 2, 3), inputConnection)

			laterMetric := factories.NewContainerMetric(appID, 0, 20, 2, 3)
			send(laterMetric, inputConnection)

			time.Sleep(5 * time.Second)

			receivedChan = make(chan []byte)
			ws, _ := AddWSSink(receivedChan, "4567", "/apps/"+appID+"/containermetrics")
			defer ws.Close()

			var receivedMessageBytes []byte
			Eventually(receivedChan).Should(Receive(&receivedMessageBytes))

			receivedEnvelope := UnmarshalMessage(receivedMessageBytes)

			Expect(receivedEnvelope.GetContainerMetric()).To(Equal(laterMetric))
		})
	}

	Context("TLS", func() {
		BeforeEach(func() {
			var err error
			inputConnection, err = DialTLS(localIPAddress+":8766", "../fixtures/client.crt", "../fixtures/client.key", "../fixtures/loggregator-ca.crt")
			Expect(err).NotTo(HaveOccurred())

			guid, _ := uuid.NewV4()
			appID = guid.String()
		})

		AfterEach(func() {
			inputConnection.Close()
		})

		itDoesContainerMetrics(SendEventTCP)
	})

	Context("UDP", func() {
		BeforeEach(func() {
			inputConnection, _ = net.Dial("udp", localIPAddress+":8765")
			guid, _ := uuid.NewV4()
			appID = guid.String()
		})

		AfterEach(func() {
			inputConnection.Close()
		})

		itDoesContainerMetrics(SendEvent)
	})
})
