package main

import (
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/cloudfoundry/noaa/consumer"
	"github.com/cloudfoundry/sonde-go/events"
	uuid "github.com/nu7hatch/gouuid"
)

const (
	defaultSampleSize   = 10
	readAttempts        = 5
	readAttemptDuration = time.Second
	messagePrefix       = "loggregator-latency-test-"
)

func main() {
	log.SetOutput(os.Stdout)

	addr, token, location, origin, err := input()
	if err != nil {
		log.Fatal(err)
	}

	mux := &http.ServeMux{}
	mux.Handle("/", &healthHandler{})
	mux.Handle("/latency", &latencyHandler{
		location: location,
		origin:   origin,
		token:    token,
	})

	server := &http.Server{
		Addr:           addr,
		Handler:        mux,
		ReadTimeout:    5 * time.Second,
		WriteTimeout:   10 * time.Second,
		MaxHeaderBytes: 1 << 20,
	}
	log.Print("listening on " + addr)
	log.Fatal(server.ListenAndServe())
}

func input() (addr, token string, location, origin *url.URL, err error) {
	targetURL := os.Getenv("TARGET_URL")
	if targetURL == "" {
		return "", "", nil, nil, errors.New("empty target url")
	}

	token = os.Getenv("TOKEN")
	if token == "" {
		return "", "", nil, nil, errors.New("empty token")
	}

	port := os.Getenv("PORT")
	if port == "" {
		return "", "", nil, nil, errors.New("empty port")
	}
	addr = ":" + port

	location, err = url.Parse(targetURL)
	if err != nil {
		return "", "", nil, nil, fmt.Errorf("invalid target url: %s", err)
	}
	// This shallow copies location, which is enough to create an origin URL.
	originLocal := *location

	switch location.Scheme {
	case "ws":
		originLocal.Scheme = "http"
	case "wss":
		originLocal.Scheme = "https"
	default:
		return "", "", nil, nil, errors.New("target url requires a scheme of ws or wss")
	}
	origin = &originLocal

	return addr, token, location, origin, nil
}

func appID() (string, error) {
	appJSON := []byte(os.Getenv("VCAP_APPLICATION"))
	var appData map[string]interface{}
	err := json.Unmarshal(appJSON, &appData)
	if err != nil {
		return "", err
	}
	appID, ok := appData["application_id"].(string)
	if !ok {
		return "", errors.New("can not type assert app id")
	}
	return appID, nil
}

type healthHandler struct{}

func (h *healthHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
}

type latencyHandler struct {
	location *url.URL
	origin   *url.URL
	token    string
}

func (h *latencyHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	sampleSize := sampleSize(r)

	avg, err := h.executeLatencyTest(sampleSize)
	if err != nil {
		log.Printf("sample failed: %s", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	fmt.Fprintf(w, "%f\n", avg.Seconds())
}

func sampleSize(r *http.Request) int {
	samplesQuery := r.URL.Query().Get("samples")
	if samplesQuery == "" {
		return defaultSampleSize
	}
	sampleSize, err := strconv.Atoi(samplesQuery)
	if err != nil || sampleSize < 1 || sampleSize > 1000 {
		return defaultSampleSize
	}
	return sampleSize
}

func (h *latencyHandler) executeLatencyTest(sampleQuantity int) (time.Duration, error) {
	sendTimes := make(map[string]time.Time)
	results := make(map[string]time.Duration)

	appID, _ := appID()
	consumer := consumer.New(h.location.String(), &tls.Config{InsecureSkipVerify: true}, nil)
	consumer.SetDebugPrinter(ConsoleDebugPrinter{})
	msgChan, errorChan := consumer.Stream(appID, h.token)
	defer consumer.Close()

	go func() {
		for err := range errorChan {
			if err == nil {
				return
			}
			log.Println(err)
		}
	}()

Loop:
	for {
		select {
		case envelope := <-msgChan:
			if envelope.GetEventType() == events.Envelope_LogMessage {
				message := string(envelope.GetLogMessage().GetMessage())

				if message == "ENSURE CONNECTION" {
					break Loop
				}
			}
		default:
			fmt.Println("ENSURE CONNECTION")
			time.Sleep(250 * time.Millisecond)
		}
	}

	var mutex sync.RWMutex
	go func() {
		for i := 0; i < sampleQuantity; i++ {
			sampleMessage := generateRandomMessage()
			mutex.Lock()
			sendTimes[sampleMessage] = time.Now()
			fmt.Println(sampleMessage)
			mutex.Unlock()
			time.Sleep(1 * time.Millisecond)
		}
	}()

	timer := time.NewTimer(readAttemptDuration)
	for {
		select {
		case envelope := <-msgChan:
			end := time.Now()

			if envelope.GetEventType() == events.Envelope_LogMessage {
				message := string(envelope.GetLogMessage().GetMessage())

				if strings.Contains(message, messagePrefix) {
					mutex.RLock()
					start, ok := sendTimes[message]
					mutex.RUnlock()

					if ok {
						results[message] = end.Sub(start)
					}
				}
			}

			if len(results) == sampleQuantity {
				return computeAverages(results)
			}

			if !timer.Stop() {
				<-timer.C
			}
			timer.Reset(readAttemptDuration)
		case <-timer.C:
			lost := sampleQuantity - len(results)
			log.Printf("Test timeout expired with %d messages lost.", lost)
			return computeAverages(results)
		}
	}
}

func computeAverages(results map[string]time.Duration) (time.Duration, error) {
	if len(results) == 0 {
		return 0, errors.New("No results.")
	}

	var totalDuration time.Duration
	for _, d := range results {
		totalDuration += d
	}

	return totalDuration / time.Duration(len(results)), nil
}

func generateRandomMessage() string {
	id, _ := uuid.NewV4()
	return fmt.Sprint(messagePrefix, id.String())
}

type ConsoleDebugPrinter struct{}

func (c ConsoleDebugPrinter) Print(title, dump string) {
	println(title)
	println(dump)
}
