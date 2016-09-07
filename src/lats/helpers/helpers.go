package helpers

import (
	"crypto/tls"

	. "github.com/onsi/gomega"

	"fmt"
	. "lats/config"
	"net"
	"strconv"
	"time"

	"github.com/cloudfoundry/noaa/consumer"
	"github.com/cloudfoundry/sonde-go/events"
)

const ORIGIN_NAME = "LATs"

var config *TestConfig

func Initialize(testConfig *TestConfig) {
	config = testConfig
}

func ConnectToFirehose() (<-chan *events.Envelope, <-chan error) {
	connection, printer := SetUpConsumer()
	randomString := strconv.FormatInt(time.Now().UnixNano(), 10)
	subscriptionId := "firehose-" + randomString[len(randomString)-5:]

	msgChan, errorChan := connection.Firehose(subscriptionId, "")

	readErrs := func() error {
		select {
		case err := <-errorChan:
			return err
		default:
			return nil
		}
	}

	Consistently(readErrs).Should(BeNil())
	WaitForWebsocketConnection(printer)

	return msgChan, errorChan
}

func SetUpConsumer() (*consumer.Consumer, *TestDebugPrinter) {
	tlsConfig := tls.Config{InsecureSkipVerify: config.SkipSSLVerify}
	printer := &TestDebugPrinter{}

	connection := consumer.New(config.DopplerEndpoint, &tlsConfig, nil)
	connection.SetDebugPrinter(printer)
	return connection, printer
}

func WaitForWebsocketConnection(printer *TestDebugPrinter) {
	Eventually(printer.Dump, 2*time.Second).Should(ContainSubstring("101 Switching Protocols"))
}

func EmitToMetron(envelope *events.Envelope) {
	metronConn, err := net.Dial("udp4", fmt.Sprintf("localhost:%d", config.DropsondePort))
	Expect(err).NotTo(HaveOccurred())

	b, err := envelope.Marshal()
	Expect(err).NotTo(HaveOccurred())

	_, err = metronConn.Write(b)
	Expect(err).NotTo(HaveOccurred())
}

func FindMatchingEnvelope(msgChan <-chan *events.Envelope) *events.Envelope {
	timeout := time.After(10 * time.Second)
	for {
		select {
		case receivedEnvelope := <-msgChan:
			if receivedEnvelope.GetOrigin() == ORIGIN_NAME {
				return receivedEnvelope
			}
		case <-timeout:
			return nil
		}
	}
}
