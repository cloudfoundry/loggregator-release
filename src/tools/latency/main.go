package main

import (
	"bytes"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"time"

	"github.com/gorilla/websocket"
)

const (
	defaultSampleSize   = 10
	readAttempts        = 5
	readAttemptDuration = 400 * time.Millisecond
)

var dialer = websocket.Dialer{
	HandshakeTimeout: 5 * time.Second,
}

func main() {
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

	appID, err := appID()
	if err != nil {
		return "", "", nil, nil, fmt.Errorf("unablet to obtain app id: %s", err)
	}

	location, err = url.Parse(targetURL)
	if err != nil {
		return "", "", nil, nil, fmt.Errorf("invalid target url: %s", err)
	}
	// This shallow copies location, which is enough to create an origin URL.
	originLocal := *location
	location.Path = fmt.Sprintf("/apps/%s/stream", appID)

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
	var samples []time.Duration

	for i := 0; i < sampleSize; i++ {
		sample, err := h.sample()
		if err != nil {
			log.Printf("sample failed: %s", err)
			continue
		}
		samples = append(samples, sample)
	}

	if len(samples) < 1 || len(samples) < sampleSize/2 {
		log.Printf("not enough samples to average")
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	fmt.Fprintf(w, "%f\n", average(samples).Seconds())
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

func average(samples []time.Duration) time.Duration {
	var total uint64
	for _, sample := range samples {
		total += uint64(sample)
	}
	return time.Duration(total / uint64(len(samples)))
}

func (h *latencyHandler) sample() (time.Duration, error) {
	conn, err := h.establishConnection()
	if err != nil {
		return 0, fmt.Errorf("dial error: %s", err)
	}
	defer conn.Close()

	msg, err := generateRandomMessage()
	if err != nil {
		return 0, err
	}
	var outMsg []byte
	outMsg = append(outMsg, msg...)
	outMsg = append(outMsg, []byte("\n")...)

	start := time.Now()
	n, err := os.Stdout.Write(outMsg)
	if err != nil {
		return 0, err
	}
	if n != len(outMsg) {
		return 0, fmt.Errorf("written length: %d does not match length: %d", n, len(outMsg))
	}

	var (
		end   time.Time
		found bool
		b     []byte
	)
	for i := 0; i < readAttempts; i++ {
		conn.SetReadDeadline(time.Now().Add(readAttemptDuration))
		_, b, err = conn.ReadMessage()
		end = time.Now()
		if err != nil {
			continue
		}
		if bytes.Contains(b, msg) {
			found = true
			break
		}
		log.Printf("unexpected message")
	}
	if err != nil {
		return 0, err
	}
	if !found {
		return 0, errors.New("test message was not found")
	}

	return end.Sub(start), nil
}

func (h *latencyHandler) establishConnection() (*websocket.Conn, error) {
	header := make(http.Header)
	header.Add("Authorization", h.token)
	header.Add("Origin", h.origin.String())
	conn, _, err := dialer.Dial(h.location.String(), header)
	return conn, err
}

func generateRandomMessage() ([]byte, error) {
	r, err := randString()
	if err != nil {
		return nil, err
	}
	return []byte("loggregator-latency-test-" + r), nil
}

func randString() (string, error) {
	b := make([]byte, 20)
	_, err := rand.Read(b)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", b), nil
}
