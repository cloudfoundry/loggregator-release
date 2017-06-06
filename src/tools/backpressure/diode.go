package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	gendiodes "github.com/cloudfoundry/diodes"
)

type diode struct {
	d *gendiodes.Poller
}

func newDiode() *diode {
	alerter := gendiodes.AlertFunc(func(int) {

	})
	return &diode{
		d: gendiodes.NewPoller(gendiodes.NewOneToOne(100000, alerter)),
	}
}

func (d *diode) Set(data time.Duration) {
	d.d.Set(gendiodes.GenericDataType(&data))
}

func (d *diode) TryNext() (time.Duration, bool) {
	data, ok := d.d.TryNext()
	if !ok {
		return 0, ok
	}
	return *(*time.Duration)(data), true
}

func (d *diode) Next() time.Duration {
	data := d.d.Next()
	return *(*time.Duration)(data)
}

func postDatadog(avg float64, host string) {
	url := fmt.Sprintf(
		"https://app.datadoghq.com/api/v1/series?api_key=%s",
		os.Getenv("DATADOG_API_KEY"),
	)
	payload := fmt.Sprintf(`
		{"series": [{
			"metric": "loggregator.backpressure.duration",
			"points": [[%d, %f]],
			"type": "gauge",
			"host": "%s"
		}]}
	`, time.Now().Unix(), avg, host)
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
