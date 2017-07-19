package reliability

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type HTTP interface {
	Post(url string, contentType string, body io.Reader) (resp *http.Response, err error)
}

type DataDogReporter struct {
	apiKey string
	host   string
	client HTTP
}

func NewDataDogReporter(key, host string, h HTTP) *DataDogReporter {
	return &DataDogReporter{
		apiKey: key,
		host:   host,
		client: h,
	}
}

func (r *DataDogReporter) Report(t *TestResult) error {
	resp, err := r.client.Post(
		fmt.Sprintf("https://app.datadoghq.com/api/v1/series?api_key=%s", r.apiKey),
		"application/json;charset=utf-8",
		strings.NewReader(
			buildPayload(r.host, t.TimeCompleted.Unix(), t.ReceivedLogCount, t.Cycles, t.Delay),
		),
	)
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("status code was %d", resp.StatusCode)
	}
	return nil
}

func buildPayload(host string, t int64, msgCount, cycles uint64, delay time.Duration) string {
	return fmt.Sprintf(`{
		"series": [
			{
				"metric": "smoke_test.loggregator.msg_count",
				"points": [[%[1]d, %[4]d]],
				"type": "gauge",
				"host": "%[2]s",
				"tags": ["firehose-nozzle", "delay:%[3]d"]
			},
			{
				"metric": "smoke_test.loggregator.cycles",
				"points": [[%[1]d, %[5]d]],
				"type": "gauge",
				"host": "%[2]s",
				"tags": ["firehose-nozzle", "delay:%[3]d"]
			}
		]
	}`, t, host, delay, msgCount, cycles)
}

type TestResult struct {
	ReceivedLogCount uint64
	TimeCompleted    time.Time
	Delay            time.Duration
	Cycles           uint64
}

func NewTestResult(test *Test, count uint64, t time.Time) *TestResult {
	return &TestResult{
		Cycles:           test.Cycles,
		Delay:            time.Duration(test.Delay),
		ReceivedLogCount: count,
		TimeCompleted:    t,
	}
}
