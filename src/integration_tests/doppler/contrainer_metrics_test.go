package doppler_test

import (
    "net"

    "github.com/nu7hatch/gouuid"

    . "github.com/onsi/ginkgo"
    . "github.com/onsi/gomega"
    "github.com/cloudfoundry/dropsonde/factories"
    "github.com/cloudfoundry/dropsonde/events"
)

var _ = Describe("Container Metrics", func() {
    var receivedChan chan []byte
    var inputConnection net.Conn
    var appID string
    var containerMetric *events.ContainerMetric

    BeforeEach(func() {
        inputConnection, _ = net.Dial("udp", localIPAddress+":8765")
        guid, _ := uuid.NewV4()
        appID = guid.String()

        waitChan := make(chan []byte)
        waitSocket, _ := addWSSink(waitChan, "4567", "/apps/"+appID+"/stream")

        containerMetric = factories.NewContainerMetric(appID, 0, 1, 2, 3)
        err := sendEvent(containerMetric, inputConnection)
        Expect(err).NotTo(HaveOccurred())

        // Received something in the stream websocket so now we can listen for container metrics.
        // This ensures that we don't listen for containermetrics before it is processed.
        Eventually(waitChan).Should(Receive())
        waitSocket.Close()

    })

    AfterEach(func() {
        inputConnection.Close()
    })

    It("returns container metrics for an app", func(){
        receivedChan = make(chan []byte)
        ws, _ := addWSSink(receivedChan, "4567", "/apps/"+appID+"/containermetrics")
        defer ws.Close()

        var receivedMessageBytes []byte
        Eventually(receivedChan).Should(Receive(&receivedMessageBytes))

        receivedEnvelope := UnmarshalMessage(receivedMessageBytes)

        Expect(receivedEnvelope.GetEventType()).To(Equal(events.Envelope_ContainerMetric))
        receivedMetric := receivedEnvelope.GetContainerMetric()
        Expect(receivedMetric).To(Equal(containerMetric))
    })

    It("does not recieve metrics for different appIds", func() {
        sendEvent(factories.NewContainerMetric(appID+"other", 0, 1, 2, 3), inputConnection)
        sendEvent(factories.NewContainerMetric(appID, 0, 100, 2, 3), inputConnection)
        sendEvent(factories.NewContainerMetric(appID+"other", 0, 1, 2, 3), inputConnection)

        receivedChan = make(chan []byte)
        ws, _ := addWSSink(receivedChan, "4567", "/apps/"+appID+"/containermetrics")
        defer ws.Close()

        var receivedMessageBytes []byte
        Eventually(receivedChan).Should(Receive(&receivedMessageBytes))
        Eventually(receivedChan).Should(BeClosed())

        receivedEnvelope := UnmarshalMessage(receivedMessageBytes)

        Expect(receivedEnvelope.GetContainerMetric().GetApplicationId()).To(Equal(appID))
        Expect(int(receivedEnvelope.GetContainerMetric().GetCpuPercentage())).To(Equal(100))
    })

    It("returns metrics for all instances of the app", func() {
        sendEvent(factories.NewContainerMetric(appID, 1, 1, 2, 3), inputConnection)

        receivedChan = make(chan []byte)
        ws, _ := addWSSink(receivedChan, "4567", "/apps/"+appID+"/containermetrics")
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
        sendEvent(factories.NewContainerMetric(appID, 0, 10, 2, 3), inputConnection)
        sendEvent(factories.NewContainerMetric(appID, 0, 20, 2, 3), inputConnection)

        receivedChan = make(chan []byte)
        ws, _ := addWSSink(receivedChan, "4567", "/apps/"+appID+"/containermetrics")
        defer ws.Close()

        var receivedMessageBytes []byte
        Eventually(receivedChan).Should(Receive(&receivedMessageBytes))

        receivedEnvelope := UnmarshalMessage(receivedMessageBytes)

        Expect(int(receivedEnvelope.GetContainerMetric().GetCpuPercentage())).To(Equal(20))
    })
})
