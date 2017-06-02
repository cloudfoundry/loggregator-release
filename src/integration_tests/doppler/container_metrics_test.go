package doppler_test

import (
	"net"
	"time"

	"github.com/cloudfoundry/dropsonde/factories"
	"github.com/cloudfoundry/sonde-go/events"
	uuid "github.com/nu7hatch/gouuid"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Container Metrics", func() {
	var (
		receivedChan    chan []byte
		inputConnection net.Conn
		appID           string
	)

	Context("UDP", func() {
		BeforeEach(func() {
			inputConnection, _ = net.Dial("udp", localIPAddress+":8765")
			guid, _ := uuid.NewV4()
			appID = guid.String()
		})

		AfterEach(func() {
			inputConnection.Close()
		})

		It("returns container metrics for an app", func() {
			containerMetric := factories.NewContainerMetric(appID, 0, 1, 2, 3)

			SendEvent(containerMetric, inputConnection)

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
			SendEvent(factories.NewContainerMetric(appID+"other", 0, 1, 2, 3), inputConnection)

			goodMetric := factories.NewContainerMetric(appID, 0, 100, 2, 3)
			SendEvent(goodMetric, inputConnection)

			SendEvent(factories.NewContainerMetric(appID+"other", 1, 1, 2, 3), inputConnection)

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

		XIt("returns metrics for all instances of the app", func() {
			SendEvent(factories.NewContainerMetric(appID, 0, 1, 2, 3), inputConnection)
			SendEvent(factories.NewContainerMetric(appID, 1, 1, 2, 3), inputConnection)

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
			SendEvent(factories.NewContainerMetric(appID, 0, 10, 2, 3), inputConnection)

			laterMetric := factories.NewContainerMetric(appID, 0, 20, 2, 3)
			SendEvent(laterMetric, inputConnection)

			time.Sleep(5 * time.Second)

			receivedChan = make(chan []byte)
			ws, _ := AddWSSink(receivedChan, "4567", "/apps/"+appID+"/containermetrics")
			defer ws.Close()

			var receivedMessageBytes []byte
			Eventually(receivedChan).Should(Receive(&receivedMessageBytes))

			receivedEnvelope := UnmarshalMessage(receivedMessageBytes)

			Expect(receivedEnvelope.GetContainerMetric()).To(Equal(laterMetric))
		})
	})
})
