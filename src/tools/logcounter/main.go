// tool to keep track of messages sent for all deployed apps

// usage: . setup.sh && go run main.go

package main

import (
	"bytes"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/cloudfoundry/noaa/consumer"
	"github.com/cloudfoundry/sonde-go/events"
	"github.com/nu7hatch/gouuid"
)

var (
	apiAddress     = os.Getenv("API_URL")
	dopplerAddress = os.Getenv("DOPPLER_URL")
	uaaAddress     = os.Getenv("UAA_URL")
	clientID       = os.Getenv("CLIENT_ID")
	clientSecret   = os.Getenv("CLIENT_SECRET")
	username       = os.Getenv("CF_USERNAME")
	password       = os.Getenv("CF_PASSWORD")
	messagePrefix  = os.Getenv("MESSAGE_PREFIX")
	subscriptionId = os.Getenv("SUBSCRIPTION_ID")

	counterWG              sync.WaitGroup
	counterLock            sync.Mutex
	counters               = make(map[Identity]map[string]bool)
	firehoseSubscriptionId = func() string {
		guid, err := uuid.NewV4()
		if err != nil {
			log.Fatal(err)
		}
		return guid.String()
	}()
	prefixEnd = len(messagePrefix + " guid: ")
	guidEnd   = prefixEnd + len("376ce05d-e4a7-46b2-6df4-663bd001b807")
	sepEnd    = guidEnd + len(" msg: ")
)

type Identity struct {
	appID string
	runID string
}

func init() {
	http.DefaultClient.Timeout = 10 * time.Second
	http.DefaultClient.Transport = &http.Transport{
		TLSHandshakeTimeout: time.Second,
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true,
		},
	}
}

func main() {
	start := time.Now()
	fmt.Println("start time:", start)
	defer func() {
		fmt.Println("\n\nJoining remaining goroutines")
		counterWG.Wait()
		fmt.Println("Done Joining")
		end := time.Now()
		fmt.Println("end time:", end)
		fmt.Println("duration:", end.Sub(start), "\n")
		if err := dumpReport(os.Stdout); err != nil {
			log.Printf("Error dumping final report: %s", err)
		}
	}()

	go func() {
		http.HandleFunc("/", func(rw http.ResponseWriter, r *http.Request) {
			rw.WriteHeader(200)
			r.ParseForm()
			if _, ok := r.Form["report"]; ok {
				if err := dumpReport(rw); err != nil {
					log.Printf("Error dumping report: %s", err)
				}
			}
		})
		err := http.ListenAndServe(":"+os.Getenv("PORT"), nil)
		if err != nil {
			log.Fatal(err)
		}
	}()

	consumer := consumer.New(dopplerAddress, &tls.Config{InsecureSkipVerify: true}, nil)

	fmt.Println("===== Streaming Firehose (will only succeed if you have admin credentials)")

	// notify on ctrl+c
	terminate := make(chan os.Signal, 1)
	signal.Notify(terminate, os.Interrupt)

	for {
		authToken, err := getAuthToken()
		if err != nil || authToken == "" {
			fmt.Fprintf(os.Stderr, "error getting token %s\n", err)
			continue
		}
		fmt.Println("got new oauth token")
		if subscriptionId != "" {
			firehoseSubscriptionId = subscriptionId
		}
		msgs, errors := consumer.FirehoseWithoutReconnect(firehoseSubscriptionId, authToken)

		go handleMessages(msgs)
		done := handleErrors(errors, terminate, consumer)
		if done {
			return
		}
	}
}

func getAppName(guid, authToken string) string {
	if authToken == "" {
		return guid
	}
	req, _ := http.NewRequest("GET", fmt.Sprintf("%s/v2/apps/%s", apiAddress, guid), nil)
	req.Header.Set("Authorization", fmt.Sprintf("bearer %s", authToken))
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Printf("Error getting app name: %s", err)
		return guid
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		log.Printf("Got status %s while getting app name", resp.Status)
		return guid
	}
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
	basicAuthToken := base64.StdEncoding.EncodeToString([]byte(clientID + ":" + clientSecret))
	r.Header.Set("Authorization", "Basic "+basicAuthToken)
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := http.DefaultClient.Do(r)
	if err != nil {
		return "", err
	}
	if resp.StatusCode != http.StatusOK {
		errout, _ := ioutil.ReadAll(resp.Body)
		fmt.Printf("StatuCode: %d\n Error: %s\n", resp.StatusCode, errout)
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

func handleErrors(errors <-chan error, terminate chan os.Signal, consumer *consumer.Consumer) bool {
	defer consumer.Close()
	select {
	case err := <-errors:
		fmt.Fprintf(os.Stderr, "%s\n", err)
		return false
	case <-terminate:
		return true
	}
}

func handleMessages(msgs <-chan *events.Envelope) {
	i := 0
	for msg := range msgs {
		i++
		if i%1000 == 0 {
			go fmt.Printf(".")
		}
		counterWG.Add(1)
		go processEnvelope(msg)
	}
}

func processEnvelope(env *events.Envelope) {
	defer counterWG.Done()
	if env.GetEventType() != events.Envelope_LogMessage {
		return
	}
	logMsg := env.GetLogMessage()

	msg := string(logMsg.GetMessage())
	if strings.HasPrefix(msg, "mismatched prefix") {
		return
	}
	if !strings.HasPrefix(msg, messagePrefix) {
		fmt.Printf("mismatched prefix: log message %s did not match prefix: %s\n", string(logMsg.GetMessage()), string(messagePrefix))
		return
	}

	if len(msg) < sepEnd {
		fmt.Printf("Cannot parse message %s\n", msg)
		return
	}

	id := Identity{
		appID: logMsg.GetAppId(),
		runID: msg[prefixEnd:guidEnd],
	}

	counterLock.Lock()
	defer counterLock.Unlock()
	counter, ok := counters[id]
	if !ok {
		counter = make(map[string]bool)
		counters[id] = counter
	}
	counter[msg[sepEnd:]] = true
}

func dumpReport(w io.Writer) error {
	counterLock.Lock()
	defer counterLock.Unlock()
	authToken, err := getAuthToken()
	if err != nil {
		authToken = ""
	}
	if _, err := io.WriteString(w, "Report:\n"); err != nil {
		return err
	}
	if len(counters) == 0 {
		_, err := io.WriteString(w, "No messages received")
		return err
	}
	for id, messages := range counters {
		var total, max int
		for msgID := range messages {
			msgMax, err := strconv.Atoi(msgID)
			if err != nil {
				if _, err := io.WriteString(w, fmt.Sprintf("Cannot parse message ID %s\n", msgID)); err != nil {
					return err
				}
				continue
			}
			if msgMax > max {
				max = msgMax
			}
			total++
		}
		msg := fmt.Sprintf("guid: %s app: %s total: %d max: %d\n", id.runID, getAppName(id.appID, authToken), total, max+1)
		if _, err := io.WriteString(w, msg); err != nil {
			return err
		}
	}
	return nil
}
