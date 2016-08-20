package helpers

import (
	"crypto/tls"

	. "github.com/onsi/gomega"

	"encoding/json"
	"fmt"
	. "lats/config"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/cloudfoundry/noaa"
	"github.com/cloudfoundry/sonde-go/events"
)

const ORIGIN_NAME = "LATs"

var config *TestConfig

func Initialize(testConfig *TestConfig) {
	config = testConfig
}

func ConnectToFirehose() (chan *events.Envelope, chan error) {
	msgChan := make(chan *events.Envelope)
	errorChan := make(chan error)
	authToken := GetAuthToken()

	connection, printer := SetUpConsumer()
	randomString := strconv.FormatInt(time.Now().UnixNano(), 10)
	subscriptionId := "firehose-" + randomString[len(randomString)-5:]

	go connection.Firehose(subscriptionId, authToken, msgChan, errorChan)

	Consistently(errorChan).ShouldNot(Receive())
	WaitForWebsocketConnection(printer)

	return msgChan, errorChan
}

func SetUpConsumer() (*noaa.Consumer, *TestDebugPrinter) {
	tlsConfig := tls.Config{InsecureSkipVerify: config.SkipSSLVerify}
	printer := &TestDebugPrinter{}

	connection := noaa.NewConsumer(config.DopplerEndpoint, &tlsConfig, nil)
	connection.SetDebugPrinter(printer)
	return connection, printer
}

func GetAuthToken() string {
	if config.LoginRequired {
		token, err := adminLogin()
		Expect(err).NotTo(HaveOccurred())
		return token
	} else {
		return ""
	}
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

func adminLogin() (string, error) {
	data := url.Values{
		"username":   {config.AdminUser},
		"password":   {config.AdminPassword},
		"client_id":  {"cf"},
		"grant_type": {"password"},
		"scope":      {},
	}

	request, err := http.NewRequest("POST", fmt.Sprintf("%s/oauth/token", config.UaaURL), strings.NewReader(data.Encode()))
	if err != nil {
		return "", err
	}
	request.SetBasicAuth("cf", "")
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	tlsConfig := &tls.Config{InsecureSkipVerify: config.SkipSSLVerify}
	tr := &http.Transport{TLSClientConfig: tlsConfig}
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

func FindMatchingEnvelope(msgChan chan *events.Envelope) *events.Envelope {
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
