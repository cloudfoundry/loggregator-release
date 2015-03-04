package doppler_test

import (
    . "github.com/onsi/ginkgo"
    . "github.com/onsi/gomega"
    "github.com/nu7hatch/gouuid"
    "net"
    "time"
    "github.com/cloudfoundry/dropsonde/events"
    "github.com/cloudfoundry/dropsonde/factories"
    "github.com/gorilla/websocket"
)

var _ = Describe("Firehose test", func() {
    var inputConnection net.Conn
    var appID string

    BeforeEach(func() {

        guid, _ := uuid.NewV4()
        appID = guid.String()

        inputConnection, _ = net.Dial("udp", localIPAddress+":8765")
        time.Sleep(50 * time.Millisecond) // give time for connection to establish
    })

    AfterEach(func() {
        inputConnection.Close()
    })
    Context("a single firehose gets all types of logs", func() {
        var ws *websocket.Conn
        var receiveChan chan []byte
        BeforeEach(func() {
            receiveChan = make(chan []byte, 10)
            ws, _ = addWSSink(receiveChan, "4567", "/firehose/hose-subcription-a")
        })
        AfterEach(func() {
            ws.Close()
        })
        It("receives log messages", func() {
            sendAppLog(appID, "message", inputConnection)

            receivedMessageBytes := []byte{}
            Eventually(receiveChan).Should(Receive(&receivedMessageBytes))

            receivedMessage := decodeProtoBufLogMessage(receivedMessageBytes)
            Expect(*receivedMessage).To(BeAssignableToTypeOf(events.LogMessage{}))
        })

        It("receives container metrics", func() {
            containerMetric := factories.NewContainerMetric(appID, 0, 10, 2, 3)
            sendEvent(containerMetric, inputConnection)

            receivedMessageBytes := []byte{}
            Eventually(receiveChan).Should(Receive(&receivedMessageBytes))

            receivedMessage := UnmarshalMessage(receivedMessageBytes)
            Expect(*receivedMessage.ContainerMetric).To(BeAssignableToTypeOf(events.ContainerMetric{}))
        })

        It("receives system status logs", func() {
            sendEvent(factories.NewHeartbeat(10, 6, 4), inputConnection)

            receivedMessageBytes := []byte{}
            Eventually(receiveChan).Should(Receive(&receivedMessageBytes))

            receivedMessage := UnmarshalMessage(receivedMessageBytes)
            Expect(*receivedMessage.Heartbeat).To(BeAssignableToTypeOf(events.Heartbeat{}))
        })

    })

    It("two separate firehose subscriptions receive the same message", func() {
        receiveChan1 := make(chan []byte, 10)
        receiveChan2 := make(chan []byte, 10)
        firehoseWs1, _ := addWSSink(receiveChan1, "4567", "/firehose/hose-subscription-1")
        firehoseWs2, _ := addWSSink(receiveChan2, "4567", "/firehose/hose-subscription-2")
        defer firehoseWs1.Close()
        defer firehoseWs2.Close()

        sendAppLog(appID, "message", inputConnection)

        receivedMessageBytes1 := []byte{}
        Eventually(receiveChan1).Should(Receive(&receivedMessageBytes1))

        receivedMessageBytes2 := []byte{}
        Eventually(receiveChan2).Should(Receive(&receivedMessageBytes2))

        receivedMessage1 := decodeProtoBufLogMessage(receivedMessageBytes1)
        Expect(string(receivedMessage1.GetMessage())).To(Equal("message"))

        receivedMessage2 := decodeProtoBufLogMessage(receivedMessageBytes2)
        Expect(string(receivedMessage2.GetMessage())).To(Equal("message"))
    })

    It("firehose subscriptions split message load", func() {
        receiveChan1 := make(chan []byte, 10)
        receiveChan2 := make(chan []byte, 10)
        firehoseWs1, _ := addWSSink(receiveChan1, "4567", "/firehose/hose-subscription-1")
        firehoseWs2, _ := addWSSink(receiveChan2, "4567", "/firehose/hose-subscription-1")
        defer firehoseWs1.Close()
        defer firehoseWs2.Close()

        for i := 0; i < 10; i++ {
            sendAppLog(appID, "message", inputConnection)
        }

        for j := 0; j < 5; j++ {
            Eventually(receiveChan1).Should(Receive())
        }

        for k := 0; k < 5; k++ {
            Eventually(receiveChan2).Should(Receive())
        }
    })
})
