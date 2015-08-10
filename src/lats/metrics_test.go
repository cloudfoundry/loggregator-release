package lats_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/cloudfoundry/noaa"
	"crypto/tls"
	"lats/helpers"
	"github.com/cloudfoundry/sonde-go/events"
	"fmt"
	"net/url"
	"net/http"
	"strings"
	"encoding/json"
	"github.com/gogo/protobuf/proto"
	"time"
	"net"
)

const (
	ORIGIN_NAME = "LATs"
)

var (
	counterEvent = &events.CounterEvent{
		Name: proto.String("LATs-Counter"),
		Delta: proto.Uint64(5),
		Total: proto.Uint64(5),
	}
)

var _ = Describe("Sending metrics through loggregator", func() {
	Context("When a counter event is emitted to metron", func(){
		It("Gets delivered to firehose", func(){
			msgChan := make(chan *events.Envelope)
			errorChan := make(chan error)

			authToken := getAuthToken()

			connection, printer := setUpConsumer()
			go connection.Firehose("firehose-a", authToken, msgChan, errorChan)

			Consistently(errorChan).ShouldNot(Receive())
			waitForWebsocketConnection(printer)

			envelope := createCounterEvent()
			emitToMetron(envelope)

			receivedEnvelope := findMatchingEnvelope(msgChan)
			Expect(receivedEnvelope).NotTo(BeNil())

			receivedCounterEvent := receivedEnvelope.GetCounterEvent()
			Expect(receivedCounterEvent).To(Equal(counterEvent))

			emitToMetron(envelope)

			receivedEnvelope = findMatchingEnvelope(msgChan)
			Expect(receivedEnvelope).NotTo(BeNil())

			receivedCounterEvent = receivedEnvelope.GetCounterEvent()
			Expect(receivedCounterEvent.GetTotal()).To(Equal(uint64(10)))

			Expect(errorChan).To(BeEmpty())
		})

	})
})

func setUpConsumer() (*noaa.Consumer, *helpers.TestDebugPrinter) {
	tlsConfig := tls.Config{InsecureSkipVerify: config.SkipSSLVerify}
	printer := &helpers.TestDebugPrinter{}

	connection := noaa.NewConsumer(config.DopplerEndpoint, &tlsConfig, nil)
	connection.SetDebugPrinter(printer)
	return connection, printer
}

func getAuthToken() string {
	if config.LoginRequired {
		token, err := adminLogin()
		Expect(err).NotTo(HaveOccurred())
		return token
	} else {
		return ""
	}
}

func waitForWebsocketConnection(printer *helpers.TestDebugPrinter) {
	Eventually(printer.Dump).Should(ContainSubstring("101 Switching Protocols"))
}

func createCounterEvent() *events.Envelope {
	return &events.Envelope{
		Origin: proto.String(ORIGIN_NAME),
		EventType: events.Envelope_CounterEvent.Enum(),
		Timestamp: proto.Int64(time.Now().UnixNano()),
		CounterEvent: counterEvent,
	}
}

func emitToMetron(envelope *events.Envelope) {
	metronConn, err := net.Dial("udp4", fmt.Sprintf("localhost:%d", config.DropsondePort))
	Expect(err).NotTo(HaveOccurred())

	b, err := envelope.Marshal()
	Expect(err).NotTo(HaveOccurred())

	_, err = metronConn.Write(b)
	Expect(err).NotTo(HaveOccurred())
}

func findMatchingEnvelope(msgChan chan *events.Envelope) *events.Envelope {
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

func adminLogin() (string, error) {
	data := url.Values{
		"username":  { config.AdminUser },
		"password": { config.AdminPassword},
		"client_id": {"cf"},
		"grant_type": {"password"},
		"scope": {},
	}

	request, err := http.NewRequest("POST", fmt.Sprintf("%s/oauth/token", config.UaaURL), strings.NewReader(data.Encode()))
	if err != nil {
		return "", err
	}
	request.SetBasicAuth("cf", "")
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	config := &tls.Config{InsecureSkipVerify: config.SkipSSLVerify}
	tr := &http.Transport{TLSClientConfig: config}
	httpClient := &http.Client{Transport: tr}

	resp, err := httpClient.Do(request)
	if err != nil {
		return "", err
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("Received a status code %v", resp.Status)
	}

	jsonData := make(map[string]interface{})
	decoder := json.NewDecoder(resp.Body)
	err = decoder.Decode(&jsonData)

	return fmt.Sprintf("%s %s", jsonData["token_type"], jsonData["access_token"]), err
}