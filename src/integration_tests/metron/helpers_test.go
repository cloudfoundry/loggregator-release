package metron_test

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"math/rand"
	"net"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"

	dopplerConfig "doppler/config"
	metronConfig "metron/config"

	"github.com/cloudfoundry/sonde-go/events"
	"github.com/gogo/protobuf/proto"

	"github.com/cloudfoundry/dropsonde/factories"
	"github.com/nu7hatch/gouuid"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
)

const (
	sharedSecret     = "test-shared-secret"
	availabilityZone = "test-availability-zone"
	jobName          = "test-job-name"
	jobIndex         = "42"
)

func setupEtcd() (func(), string) {
	By("starting etcd")
	etcdPort := rand.Intn(55536) + 10000
	etcdClientURL := fmt.Sprintf("http://localhost:%d", etcdPort)
	etcdDataDir, err := ioutil.TempDir("", "etcd-data")
	Expect(err).ToNot(HaveOccurred())

	etcdCommand := exec.Command(
		etcdPath,
		"--data-dir", etcdDataDir,
		"--listen-client-urls", etcdClientURL,
		"--advertise-client-urls", etcdClientURL,
	)
	etcdSession, err := gexec.Start(
		etcdCommand,
		gexec.NewPrefixedWriter("[o][etcd]", GinkgoWriter),
		gexec.NewPrefixedWriter("[e][etcd]", GinkgoWriter),
	)
	Expect(err).ToNot(HaveOccurred())

	By("waiting for etcd to respond via http")
	Eventually(func() error {
		req, reqErr := http.NewRequest("PUT", etcdClientURL+"/v2/keys/test", strings.NewReader("value=test"))
		if reqErr != nil {
			return reqErr
		}
		resp, reqErr := http.DefaultClient.Do(req)
		if reqErr != nil {
			return reqErr
		}
		defer resp.Body.Close()
		if resp.StatusCode == http.StatusInternalServerError {
			return errors.New(fmt.Sprintf("got %d response from etcd", resp.StatusCode))
		}
		return nil
	}, 10).Should(Succeed())

	return func() {
		os.RemoveAll(etcdDataDir)
		etcdSession.Kill()
	}, etcdClientURL
}

func setupDoppler(etcdClientURL string) (func(), int) {
	By("starting doppler")
	dopplerUDPPort := rand.Intn(55536) + 10000
	dopplerTCPPort := rand.Intn(55536) + 10000
	dopplerTlsPort := rand.Intn(55536) + 10000
	dopplerOutgoingPort := rand.Intn(55536) + 10000

	dopplerConf := dopplerConfig.Config{
		IncomingUDPPort:    uint32(dopplerUDPPort),
		IncomingTCPPort:    uint32(dopplerTCPPort),
		OutgoingPort:       uint32(dopplerOutgoingPort),
		EtcdUrls:           []string{etcdClientURL},
		EnableTLSTransport: true,
		TLSListenerConfig: dopplerConfig.TLSListenerConfig{
			Port:     uint32(dopplerTlsPort),
			CertFile: "../fixtures/server.crt",
			KeyFile:  "../fixtures/server.key",
			CAFile:   "../fixtures/loggregator-ca.crt",
		},
		MaxRetainedLogMessages:       10,
		MessageDrainBufferSize:       100,
		SinkDialTimeoutSeconds:       10,
		SinkIOTimeoutSeconds:         10,
		SinkInactivityTimeoutSeconds: 10,
		UnmarshallerCount:            5,
		Index:                        jobIndex,
		JobName:                      jobName,
		SharedSecret:                 sharedSecret,
		Zone:                         availabilityZone,
	}

	dopplerCfgFile, err := ioutil.TempFile("", "doppler-config")
	Expect(err).ToNot(HaveOccurred())

	err = json.NewEncoder(dopplerCfgFile).Encode(dopplerConf)
	Expect(err).ToNot(HaveOccurred())
	err = dopplerCfgFile.Close()
	Expect(err).ToNot(HaveOccurred())

	dopplerCommand := exec.Command(dopplerPath, "--config", dopplerCfgFile.Name())
	dopplerSession, err := gexec.Start(
		dopplerCommand,
		gexec.NewPrefixedWriter("[o][doppler]", GinkgoWriter),
		gexec.NewPrefixedWriter("[e][doppler]", GinkgoWriter),
	)
	Expect(err).ToNot(HaveOccurred())

	// a terrible hack
	Eventually(dopplerSession.Buffer).Should(gbytes.Say("doppler server started"))

	By("waiting for doppler to listen")
	Eventually(func() error {
		c, reqErr := net.Dial("tcp", fmt.Sprintf(":%d", dopplerOutgoingPort))
		if reqErr == nil {
			c.Close()
		}
		return reqErr
	}, 3).Should(Succeed())

	return func() {
		os.Remove(dopplerCfgFile.Name())
		dopplerSession.Kill()
	}, dopplerOutgoingPort
}

func setupMetron(etcdClientURL, proto string) (func(), int) {
	By("starting metron")
	protocols := []metronConfig.Protocol{metronConfig.Protocol(proto)}
	metronPort := rand.Intn(55536) + 10000
	metronConf := metronConfig.Config{
		Deployment:                       "deployment",
		Zone:                             availabilityZone,
		Job:                              jobName,
		Index:                            jobIndex,
		IncomingUDPPort:                  metronPort,
		EtcdUrls:                         []string{etcdClientURL},
		SharedSecret:                     sharedSecret,
		MetricBatchIntervalMilliseconds:  10,
		RuntimeStatsIntervalMilliseconds: 10,
		EtcdMaxConcurrentRequests:        10,
		Protocols:                        metronConfig.Protocols(protocols),
	}

	switch proto {
	case "udp":
	case "tls":
		metronConf.TLSConfig = metronConfig.TLSConfig{
			CertFile: "../fixtures/client.crt",
			KeyFile:  "../fixtures/client.key",
			CAFile:   "../fixtures/loggregator-ca.crt",
		}
		fallthrough
	case "tcp":
		metronConf.TCPBatchIntervalMilliseconds = 100
		metronConf.TCPBatchSizeBytes = 10240
	}

	metronCfgFile, err := ioutil.TempFile("", "metron-config")
	Expect(err).ToNot(HaveOccurred())

	err = json.NewEncoder(metronCfgFile).Encode(metronConf)
	Expect(err).ToNot(HaveOccurred())
	err = metronCfgFile.Close()
	Expect(err).ToNot(HaveOccurred())

	metronCommand := exec.Command(metronPath, "--debug", "--config", metronCfgFile.Name())
	metronSession, err := gexec.Start(
		metronCommand,
		gexec.NewPrefixedWriter("[o][metron]", GinkgoWriter),
		gexec.NewPrefixedWriter("[e][metron]", GinkgoWriter),
	)
	Expect(err).ToNot(HaveOccurred())

	Eventually(metronSession.Buffer).Should(gbytes.Say(" from last etcd event, updating writer..."))

	By("waiting for metron to listen")
	Eventually(func() error {
		c, reqErr := net.Dial("udp4", fmt.Sprintf(":%d", metronPort))
		if reqErr == nil {
			c.Close()
		}
		return reqErr
	}, 3).Should(Succeed())

	return func() {
		os.Remove(metronCfgFile.Name())
		metronSession.Kill()
	}, metronPort
}

func basicValueMetric(name string, value float64, unit string) *events.ValueMetric {
	return &events.ValueMetric{
		Name:  proto.String(name),
		Value: proto.Float64(value),
		Unit:  proto.String(unit),
	}
}

func addDefaultTags(envelope *events.Envelope) *events.Envelope {
	envelope.Deployment = proto.String("deployment-name")
	envelope.Job = proto.String("test-component")
	envelope.Index = proto.String("42")
	envelope.Ip = proto.String(localIPAddress)

	return envelope
}

func basicValueMessage() []byte {
	message, _ := proto.Marshal(basicValueMessageEnvelope())
	return message
}

func basicValueMessageEnvelope() *events.Envelope {
	return &events.Envelope{
		Origin:    proto.String("fake-origin-2"),
		EventType: events.Envelope_ValueMetric.Enum(),
		ValueMetric: &events.ValueMetric{
			Name:  proto.String("fake-metric-name"),
			Value: proto.Float64(42),
			Unit:  proto.String("fake-unit"),
		},
	}
}

func basicCounterEventMessage() []byte {
	message, _ := proto.Marshal(&events.Envelope{
		Origin:    proto.String("fake-origin-2"),
		EventType: events.Envelope_CounterEvent.Enum(),
		CounterEvent: &events.CounterEvent{
			Name:  proto.String("fake-counter-event-name"),
			Delta: proto.Uint64(1),
		},
	})

	return message
}

func basicHTTPStartMessage(requestId string) []byte {
	message, _ := proto.Marshal(basicHTTPStartMessageEnvelope(requestId))
	return message
}

func basicHTTPStartMessageEnvelope(requestId string) *events.Envelope {
	uuid, _ := uuid.ParseHex(requestId)
	return &events.Envelope{
		Origin:    proto.String("fake-origin-2"),
		EventType: events.Envelope_HttpStart.Enum(),
		HttpStart: &events.HttpStart{
			Timestamp:     proto.Int64(12),
			RequestId:     factories.NewUUID(uuid),
			PeerType:      events.PeerType_Client.Enum(),
			Method:        events.Method_GET.Enum(),
			Uri:           proto.String("some uri"),
			RemoteAddress: proto.String("some address"),
			UserAgent:     proto.String("some user agent"),
		},
	}
}

func basicHTTPStopMessage(requestId string) []byte {
	message, _ := proto.Marshal(basicHTTPStopMessageEnvelope(requestId))
	return message
}

func basicHTTPStopMessageEnvelope(requestId string) *events.Envelope {
	uuid, _ := uuid.ParseHex(requestId)
	return &events.Envelope{
		Origin:    proto.String("fake-origin-2"),
		EventType: events.Envelope_HttpStop.Enum(),
		HttpStop: &events.HttpStop{
			Timestamp:     proto.Int64(12),
			Uri:           proto.String("some uri"),
			RequestId:     factories.NewUUID(uuid),
			PeerType:      events.PeerType_Client.Enum(),
			StatusCode:    proto.Int32(404),
			ContentLength: proto.Int64(98475189),
		},
	}
}

func eventsLogMessage(appID int, message string, timestamp time.Time) *events.Envelope {
	return &events.Envelope{
		Origin:    proto.String("legacy"),
		EventType: events.Envelope_LogMessage.Enum(),
		LogMessage: &events.LogMessage{
			Message:        []byte(message),
			MessageType:    events.LogMessage_OUT.Enum(),
			Timestamp:      proto.Int64(timestamp.UnixNano()),
			AppId:          proto.String(string(appID)),
			SourceType:     proto.String("fake-source-id"),
			SourceInstance: proto.String("fake-source-id"),
		},
	}
}

func eventuallyListensForUDP(address string) net.PacketConn {
	var testServer net.PacketConn

	Eventually(func() error {
		var err error
		testServer, err = net.ListenPacket("udp4", address)
		return err
	}).Should(Succeed())

	return testServer
}

func eventuallyListensForTLS(address string) net.Listener {
	var listener net.Listener

	Eventually(func() error {
		var err error
		listener, err = net.Listen("tcp", address)
		return err
	}).Should(Succeed())

	return listener
}

type MetronInput struct {
	metronConn   net.Conn
	stopTheWorld chan struct{}
}

func (input *MetronInput) WriteToMetron(unsignedMessage []byte) {
	ticker := time.NewTicker(10 * time.Millisecond)

	for {
		select {
		case <-input.stopTheWorld:
			ticker.Stop()
			return
		case <-ticker.C:
			input.metronConn.Write(unsignedMessage)
		}
	}
}

type FakeDoppler struct {
	packetConn   net.PacketConn
	stopTheWorld chan struct{}
}

func (d *FakeDoppler) ReadIncomingMessages(signature []byte) chan signedMessage {
	messageChan := make(chan signedMessage, 1000)

	go func() {
		readBuffer := make([]byte, 65535)
		for {
			select {
			case <-d.stopTheWorld:
				return
			default:
				readCount, _, _ := d.packetConn.ReadFrom(readBuffer)
				readData := make([]byte, readCount)
				copy(readData, readBuffer[:readCount])

				// Only signed messages get placed on messageChan
				if gotSignedMessage(readData, signature) {
					msg := signedMessage{
						signature: readData[:len(signature)],
						message:   readData[len(signature):],
					}
					messageChan <- msg
				}
			}
		}
	}()

	return messageChan
}

func (d *FakeDoppler) Close() {
	d.packetConn.Close()
}

type signedMessage struct {
	signature []byte
	message   []byte
}

func sign(message []byte) signedMessage {
	expectedEnvelope := addDefaultTags(basicValueMessageEnvelope())
	expectedMessage, _ := proto.Marshal(expectedEnvelope)

	mac := hmac.New(sha256.New, []byte("shared_secret"))
	mac.Write(expectedMessage)

	signature := mac.Sum(nil)
	return signedMessage{signature: signature, message: expectedMessage}
}

func gotSignedMessage(readData, signature []byte) bool {
	return len(readData) > len(signature)
}
