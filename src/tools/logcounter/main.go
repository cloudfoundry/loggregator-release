// tool to keep track of messages sent for all deployed apps

// usage: . setup.sh && go run main.go

package main

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"regexp"
	"strconv"

	"github.com/cloudfoundry/noaa/consumer"
	"github.com/cloudfoundry/sonde-go/events"
	"github.com/nu7hatch/gouuid"
)

var (
	apiAddress     = os.Getenv("API_ADDR")
	dopplerAddress = os.Getenv("DOPPLER_ADDR")
	uaaAddress     = os.Getenv("UAA_ADDR")
	clientID       = os.Getenv("CLIENT_ID")
	clientSecret   = os.Getenv("CLIENT_SECRET")
	username       = os.Getenv("USERNAME")
	password       = os.Getenv("PASSWORD")

	messageFilter  = os.Getenv("MESSAGE_FILTER")
	messagesPerApp = os.Getenv("MESSAGES_PER_APP")

	maxMessage             int
	counters               = make(map[string]map[int]bool)
	re                     = regexp.MustCompile(`msg (\d+) \w*`)
	guid, _                = uuid.NewV4()
	firehoseSubscriptionId = guid.String()
	authToken              string
)

func main() {
	var err error
	defer dumpReport()

	maxMessage, _ = strconv.Atoi(messagesPerApp)

	consumer := consumer.New(dopplerAddress, &tls.Config{InsecureSkipVerify: true}, nil)

	fmt.Println("===== Streaming Firehose (will only succeed if you have admin credentials)")

	// notify on ctrl+c
	terminate := make(chan os.Signal, 1)
	signal.Notify(terminate, os.Interrupt)

	for {
		authToken, err = getAuthToken()
		if err != nil || authToken == "" {
			fmt.Fprintf(os.Stderr, "error getting token %s\n", err)
			continue
		}
		fmt.Println("got new oauth token")
		msgChan, errorChan := consumer.Firehose(firehoseSubscriptionId, authToken)
		stop := make(chan struct{})

		go handleErrors(errorChan, stop)
		cont := handleMessages(msgChan, stop, terminate)
		if !cont {
			return
		}
	}
}

func getAppName(guid string) string {
	req, _ := http.NewRequest("GET", fmt.Sprintf("%s/v2/apps/%s", apiAddress, guid), nil)
	req.Header.Set("Authorization", fmt.Sprintf("bearer %s", authToken))
	resp, _ := http.DefaultClient.Do(req)
	defer resp.Body.Close()
	body, _ := ioutil.ReadAll(resp.Body)
	type Entity struct {
		Name string `json:"name"`
	}
	um := &struct {
		Entity Entity `json:"entity"`
	}{}

	json.Unmarshal(body, um)
	return um.Entity.Name
}

func getAuthToken() (string, error) {
	uaaURL := fmt.Sprintf("%s/oauth/token", uaaAddress)
	data := url.Values{}
	data.Set("grant_type", "password")
	data.Set("client_id", clientID)
	data.Set("client_secret", clientSecret)
	data.Set("username", username)
	data.Set("password", password)
	data.Set("response_type", "token")
	data.Set("scope", "")

	r, err := http.NewRequest("POST", uaaURL, bytes.NewBufferString(data.Encode()))
	if err != nil {
		return "", err
	}
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := http.DefaultClient.Do(r)
	if err != nil {
		return "", err
	}
	if resp.StatusCode != http.StatusOK {
		return "", errors.New("response not 200")
	}

	defer resp.Body.Close()
	content, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	um := &struct {
		AccessToken string `json:"access_token"`
	}{}
	json.Unmarshal(content, um)

	return um.AccessToken, nil
}

func handleErrors(errorChan <-chan error, stop chan struct{}) {
	for err := range errorChan {
		fmt.Fprintf(os.Stderr, "%s\n", err)
		close(stop)
		return
	}
}

func handleMessages(msgChan <-chan *events.Envelope, stop chan struct{}, terminate chan os.Signal) bool {
	for {
		select {
		case msg := <-msgChan:
			processEnvelope(msg)
		case <-stop:
			return true
		case <-terminate:
			return false
		}
	}
}

func processEnvelope(env *events.Envelope) {
	// filter out non log messages
	if env.GetEventType() != events.Envelope_LogMessage {
		return
	}
	logMsg := env.GetLogMessage()

	fmt.Printf(".")

	// filter out log messages that don't match our filter string
	if !bytes.Contains(logMsg.GetMessage(), []byte(messageFilter)) {
		fmt.Printf("log message: %s did not match filter: %s\n", string(logMsg.GetMessage()), messageFilter)
		return
	}

	// regex out the message id
	match := re.FindStringSubmatch(string(logMsg.GetMessage()))
	if match == nil {
		fmt.Printf("regex didn't match: %s\n", string(logMsg.GetMessage()))
		return
	}
	msgId := match[1]
	id, err := strconv.Atoi(msgId)
	if err != nil {
		fmt.Printf("bad message id\n")
		return
	}

	// insert message id into the set
	if counters[*logMsg.AppId] == nil {
		counters[*logMsg.AppId] = make(map[int]bool)
	}
	counters[*logMsg.AppId][id] = true
}

func dumpReport() {
	fmt.Println("\n\nReport:")
	for appId, messages := range counters {
		var total, max int
		for msgId, _ := range messages {
			if msgId >= maxMessage {
				continue
			}
			if msgId >= max {
				max = msgId
			}
			total++
		}
		fmt.Printf("guid: %s  app: %s total: %d max: %d\n", appId, getAppName(appId), total, max)
	}
}
