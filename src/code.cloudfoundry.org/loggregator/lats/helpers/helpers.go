package helpers

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"strconv"
	"time"

	"code.cloudfoundry.org/loggregator/plumbing"

	"google.golang.org/grpc"

	v2 "code.cloudfoundry.org/loggregator/plumbing/v2"

	"github.com/cloudfoundry/dropsonde/envelope_extensions"
	"github.com/cloudfoundry/noaa/consumer"
	"github.com/cloudfoundry/sonde-go/events"

	. "code.cloudfoundry.org/loggregator/lats/config"

	. "github.com/onsi/gomega"
)

const (
	OriginName = "LATs"
	WriteCount = 500
)

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

func RequestContainerMetrics(appID string) []*events.ContainerMetric {
	consumer, _ := SetUpConsumer()
	resp, _ := consumer.ContainerMetrics(appID, "")
	return resp
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

func EmitToMetronV1(envelope *events.Envelope) {
	metronConn, err := net.Dial("udp4", fmt.Sprintf("localhost:%d", config.DropsondePort))
	Expect(err).NotTo(HaveOccurred())

	b, err := envelope.Marshal()
	Expect(err).NotTo(HaveOccurred())

	_, err = metronConn.Write(b)
	Expect(err).NotTo(HaveOccurred())
}

func EmitToMetronV2(envelope *v2.Envelope) {
	creds, err := plumbing.NewClientCredentials(
		config.MetronTLSClientConfig.CertFile,
		config.MetronTLSClientConfig.KeyFile,
		config.MetronTLSClientConfig.CAFile,
		"metron",
	)
	Expect(err).NotTo(HaveOccurred())

	conn, err := grpc.Dial("localhost:3458", grpc.WithTransportCredentials(creds))
	Expect(err).NotTo(HaveOccurred())
	defer conn.Close()
	c := v2.NewIngressClient(conn)

	s, err := c.Sender(context.Background())
	Expect(err).NotTo(HaveOccurred())
	defer s.CloseSend()

	for i := 0; i < WriteCount; i++ {
		err = s.Send(envelope)
		Expect(err).NotTo(HaveOccurred())
	}
}

func ReadFromRLP(appID string, usePreferredTags bool) <-chan *v2.Envelope {
	creds, err := plumbing.NewClientCredentials(
		config.MetronTLSClientConfig.CertFile,
		config.MetronTLSClientConfig.KeyFile,
		config.MetronTLSClientConfig.CAFile,
		"reverselogproxy",
	)
	Expect(err).NotTo(HaveOccurred())

	conn, err := grpc.Dial(config.ReverseLogProxyAddr, grpc.WithTransportCredentials(creds))
	Expect(err).NotTo(HaveOccurred())

	client := v2.NewEgressClient(conn)
	receiver, err := client.Receiver(context.Background(), &v2.EgressRequest{
		UsePreferredTags: usePreferredTags,
		ShardId:          fmt.Sprint("shard-", time.Now().UnixNano()),
		Filter: &v2.Filter{
			SourceId: appID,
			Message: &v2.Filter_Log{
				Log: &v2.LogFilter{},
			},
		},
	})
	Expect(err).ToNot(HaveOccurred())

	msgChan := make(chan *v2.Envelope, 100)

	go func() {
		defer conn.Close()
		for {
			e, err := receiver.Recv()
			if err != nil {
				break
			}

			msgChan <- e
		}
	}()

	return msgChan
}

func ReadContainerFromRLP(appID string, usePreferredTags bool) []*v2.Envelope {
	creds, err := plumbing.NewClientCredentials(
		config.MetronTLSClientConfig.CertFile,
		config.MetronTLSClientConfig.KeyFile,
		config.MetronTLSClientConfig.CAFile,
		"reverselogproxy",
	)
	Expect(err).NotTo(HaveOccurred())

	conn, err := grpc.Dial(config.ReverseLogProxyAddr, grpc.WithTransportCredentials(creds))
	Expect(err).NotTo(HaveOccurred())

	client := v2.NewEgressQueryClient(conn)

	ctx, _ := context.WithTimeout(context.Background(), time.Second)
	resp, err := client.ContainerMetrics(ctx, &v2.ContainerMetricRequest{
		UsePreferredTags: usePreferredTags,
		SourceId:         appID,
	})
	Expect(err).ToNot(HaveOccurred())
	return resp.Envelopes
}

func FindMatchingEnvelope(msgChan <-chan *events.Envelope, envelope *events.Envelope) *events.Envelope {
	timeout := time.After(10 * time.Second)
	for {
		select {
		case receivedEnvelope := <-msgChan:
			if receivedEnvelope.GetTags()["UniqueName"] == envelope.GetTags()["UniqueName"] {
				return receivedEnvelope
			}
		case <-timeout:
			return nil
		}
	}
}

func FindMatchingEnvelopeByOrigin(msgChan <-chan *events.Envelope, origin string) *events.Envelope {
	timeout := time.After(10 * time.Second)
	for {
		select {
		case receivedEnvelope := <-msgChan:
			if receivedEnvelope.GetOrigin() == origin {
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
