package main

import (
	"crypto/rand"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strings"
	"time"
)

var msg1K []byte

func init() {
	msg1K = make([]byte, 1000)
	n, err := rand.Read(msg1K)
	if n != 1000 {
		log.Fatalf("1K msg not the correct length: %d", n)
	}
	if err != nil {
		log.Fatal("unable to build 1K msg")
	}
}

func postDatadog(avg float64) {
	url := fmt.Sprintf(
		"https://app.datadoghq.com/api/v1/series?api_key=%s",
		os.Getenv("DATADOG_API_KEY"),
	)
	payload := fmt.Sprintf(`
		{"series": [{
			"metric": "loggregator.backpressure.duration",
			"points": [[%d, %f]],
			"type": "gauge"
		}]}
	`, time.Now().Unix(), avg)
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

func observer(d *diode) {
	ticker := time.NewTicker(5 * time.Second)
	var count, sum float64
	for {
		select {
		case <-ticker.C:
			avg := sum / count
			sum = 0
			count = 0
			postDatadog(avg)
		default:
			delta := d.Next()
			count++
			sum += float64(delta)
		}
	}
}

func logger(d *diode) {
	for {
		start := time.Now()
		os.Stdout.Write(msg1K)
		d.Set(time.Since(start))
	}
}

func main() {
	d := newDiode()
	go observer(d)
	logger(d)
}
