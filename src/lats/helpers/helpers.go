package helpers

import (
	"context"
	"crypto/tls"
	"errors"

	"github.com/coreos/etcd/client"
	. "github.com/onsi/gomega"

	"fmt"
	. "lats/config"
	"net"
	"strconv"
	"time"

	"github.com/cloudfoundry/dropsonde/envelope_extensions"
	"github.com/cloudfoundry/noaa/consumer"
	"github.com/cloudfoundry/sonde-go/events"
)

const ORIGIN_NAME = "LATs"

var config *TestConfig

func Initialize(testConfig *TestConfig) {
	config = testConfig
}

func ConnectToStream(appID string) (<-chan *events.Envelope, <-chan error) {
	connection, printer := SetUpConsumer()
	msgChan, errorChan := connection.Stream(appID, "")

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

func RequestContainerMetrics(appID string) ([]*events.ContainerMetric, error) {
	consumer, _ := SetUpConsumer()
	return consumer.ContainerMetrics(appID, "")
}

func RequestRecentLogs(appID string) ([]*events.LogMessage, error) {
	consumer, _ := SetUpConsumer()
	return consumer.RecentLogs(appID, "")
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

func FindMatchingEnvelopeByID(id string, msgChan <-chan *events.Envelope) (*events.Envelope, error) {
	timeout := time.After(10 * time.Second)
	for {
		select {
		case receivedEnvelope := <-msgChan:
			receivedID := envelope_extensions.GetAppId(receivedEnvelope)
			if receivedID == id {
				return receivedEnvelope, nil
			}
			return nil, fmt.Errorf("Expected messages with app id: %s, got app id: %s", id, receivedID)
		case <-timeout:
			return nil, errors.New("Timed out while waiting for message")
		}
	}
}

func WriteToEtcd(urls []string, key, value string) func() {
	cfg := client.Config{
		Endpoints: urls,
	}
	c, err := client.New(cfg)
	if err != nil {
		panic(err)
	}
	api := client.NewKeysAPI(c)
	options := &client.SetOptions{TTL: time.Minute}
	api.Set(context.Background(), key, value, options)
	return func() {
		api.Delete(context.Background(), key, nil)
	}
}
