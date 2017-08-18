package reporter

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"
	sharedapi "tools/reliability/api"
)

type HTTP interface {
	Post(url string, contentType string, body io.Reader) (resp *http.Response, err error)
}

type DataDogReporter struct {
	apiKey     string
	host       string
	instanceID string
	client     HTTP
}

func NewDataDogReporter(key, host, instanceID string, h HTTP) *DataDogReporter {
	return &DataDogReporter{
		apiKey:     key,
		host:       host,
		instanceID: instanceID,
		client:     h,
	}
}

func (r *DataDogReporter) Report(t *TestResult) error {
	resp, err := r.client.Post(
		fmt.Sprintf("https://app.datadoghq.com/api/v1/series?api_key=%s", r.apiKey),
		"application/json;charset=utf-8",
		strings.NewReader(
			buildPayload(r.host, r.instanceID, t.TestStartTime, t.ReceivedLogCount, t.Cycles, t.Delay),
		),
	)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	log.Printf("datadog response status code: %d", resp.StatusCode)
	if resp.StatusCode != http.StatusCreated &&
		resp.StatusCode != http.StatusAccepted &&
		resp.StatusCode != http.StatusOK {
		return fmt.Errorf("status code was %d", resp.StatusCode)
	}

	return nil
}

func buildPayload(host, instanceID string, t int64, msgCount, cycles uint64, delay time.Duration) string {
	return fmt.Sprintf(`{
		"series": [
			{
				"metric": "smoke_test.loggregator.msg_count",
				"points": [[%[1]d, %[4]d]],
				"type": "gauge",
				"host": "%[2]s",
				"tags": ["firehose-nozzle", "delay:%[3]d", "instance_id:%[6]s"]
			},
			{
				"metric": "smoke_test.loggregator.cycles",
				"points": [[%[1]d, %[5]d]],
				"type": "gauge",
				"host": "%[2]s",
				"tags": ["firehose-nozzle", "delay:%[3]d", "instance_id:%[6]s"]
			}
		]
	}`, time.Unix(0, t).Unix(), host, delay, msgCount, cycles, instanceID)
}

type TestResult struct {
	ReceivedLogCount uint64
	Delay            time.Duration
	Cycles           uint64
	TestStartTime    int64
}

func NewTestResult(test *sharedapi.Test, count uint64) *TestResult {
	return &TestResult{
		Cycles:           test.Cycles,
		Delay:            time.Duration(test.Delay),
		ReceivedLogCount: count,
		TestStartTime:    test.StartTime,
	}
}
