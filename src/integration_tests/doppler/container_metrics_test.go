package doppler_test

import (
    "net"

    "github.com/nu7hatch/gouuid"

    . "github.com/onsi/ginkgo"
    . "github.com/onsi/gomega"
    "github.com/cloudfoundry/dropsonde/factories"
    "github.com/cloudfoundry/dropsonde/events"
)

var receivedChan chan []byte
var inputConnection net.Conn
var appID string

var _ = Describe("Container Metrics", func() {
    var metricsArray []events.ContainerMetric


    BeforeEach(func() {
        inputConnection, _ = net.Dial("udp", localIPAddress+":8765")
        guid, _ := uuid.NewV4()
        appID = guid.String()
        metricsArray = make([]events.ContainerMetric, 0)
    })

    AfterEach(func() {
        inputConnection.Close()
    })

    It("returns container metrics for an app", func(){
        containerMetric := factories.NewContainerMetric(appID, 0, 1, 2, 3)
        metricsArray = append(metricsArray, *containerMetric)
        waitForMessageToGetInDoppler(metricsArray)

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
        badMetric1 := factories.NewContainerMetric(appID+"other", 0, 1, 2, 3)
        goodMetric := factories.NewContainerMetric(appID, 0, 100, 2, 3)
        badMetric2 := factories.NewContainerMetric(appID+"other", 1, 1, 2, 3)
        metricsArray = append(metricsArray, *badMetric1, *goodMetric, *badMetric2)

        waitForMessageToGetInDoppler(metricsArray)

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
        instance1Metric := factories.NewContainerMetric(appID, 0, 1, 2, 3)
        instance2Metric := factories.NewContainerMetric(appID, 1, 1, 2, 3)
        metricsArray = append(metricsArray, *instance1Metric, *instance2Metric)

        waitForMessageToGetInDoppler(metricsArray)

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
        earlyMetric := factories.NewContainerMetric(appID, 0, 10, 2, 3)
        laterMetric := factories.NewContainerMetric(appID, 0, 20, 2, 3)
        metricsArray = append(metricsArray, *earlyMetric, *laterMetric)

        waitForMessageToGetInDoppler(metricsArray)

        receivedChan = make(chan []byte)
        ws, _ := addWSSink(receivedChan, "4567", "/apps/"+appID+"/containermetrics")
        defer ws.Close()

        var receivedMessageBytes []byte
        Eventually(receivedChan).Should(Receive(&receivedMessageBytes))

        receivedEnvelope := UnmarshalMessage(receivedMessageBytes)

        Expect(receivedEnvelope.GetContainerMetric()).To(Equal(laterMetric))
    })
})

func waitForMessageToGetInDoppler(messages []events.ContainerMetric) {
    waitChan := make(chan []byte, 5)
    waitSocket, _ := addWSSink(waitChan, "4567", "/firehose/fire-subscription-y")

    for _, containerMetric := range messages {
        err := sendEvent(&containerMetric, inputConnection)
        Expect(err).NotTo(HaveOccurred())
    }

    for i:=0; i < len(messages); i ++ {
        Eventually(waitChan).Should(Receive())
    }

    // Received the correct number of metrics in the stream websocket so now we can listen for container metrics.
    // This ensures that we don't listen for containermetrics before it is processed, as the ws Sink will otherwise
    // prematurely close
    waitSocket.Close()
}
