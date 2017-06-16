package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strings"
	"time"
)

var msg1K = []byte(strings.Repeat("J", 1024) + "\n")

func postDatadog(max float64, count int64, host string) {
	url := fmt.Sprintf(
		"https://app.datadoghq.com/api/v1/series?api_key=%s",
		os.Getenv("DATADOG_API_KEY"),
	)
	now := time.Now().Unix()
	payload := fmt.Sprintf(`
		{"series": [{
			"metric": "loggregator.backpressure.duration",
			"points": [[%d, %f]],
			"type": "gauge",
			"host": "%s"
		}, {
			"metric": "loggregator.backpressure.count",
			"points": [[%d, %d]],
			"type": "gauge",
			"host": "%s"
		}]}
	`, now, max, host, now, count, host)
	resp, err := http.Post(url, "application/json", strings.NewReader(payload))
	if err != nil {
		log.Printf("err when posting to datadog: %s", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode > http.StatusIMUsed {
		body, _ := ioutil.ReadAll(resp.Body)
		log.Printf("bad resp when posting to datadog: %d", resp.StatusCode)
		log.Printf("body: %s", string(body))
	}
}

func observer(d chan time.Duration) {
	host := hostFromEnv()
	ticker := time.NewTicker(5 * time.Second)
	var (
		max   time.Duration
		count int64
	)
	for {
		select {
		case <-ticker.C:
			report := max
			max = 0
			postDatadog(float64(report), count, host)
		case delta := <-d:
			count++
			if delta > max {
				max = delta
			}
		}
	}
}

func logger(d chan time.Duration) {
	for {
		start := time.Now()
		os.Stdout.Write(msg1K)
		select {
		case d <- time.Since(start):
		default:
		}
	}
}

func hostFromEnv() string {
	type vcap struct {
		URIs []string `json:"uris"`
	}
	vcapData := &vcap{}
	err := json.Unmarshal([]byte(os.Getenv("VCAP_APPLICATION")), vcapData)
	if err != nil {
		log.Fatalf("unable to parse VCAP_APPLICATION: %s", err)
	}
	if len(vcapData.URIs) == 0 {
		log.Fatal("no uris available from VCAP_APPLICATION")
	}
	return vcapData.URIs[0]
}

func main() {
	d := make(chan time.Duration, 100000)
	go observer(d)
	logger(d)
}
