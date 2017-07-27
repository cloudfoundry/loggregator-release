package reliability_test

import (
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"time"

	"tools/reliability"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("DataDogReporter", func() {
	It("sends a test result", func() {
		spyHTTPClient := &spyHTTPClient{}
		spyHTTPClient.postResponseReturn = &http.Response{StatusCode: 201}
		r := reliability.NewDataDogReporter(
			"somekey",
			"mycoolhost.cfapps.io",
			spyHTTPClient,
		)

		now := time.Now()
		r.Report(&reliability.TestResult{
			Delay:            1 * time.Second,
			Cycles:           54321,
			ReceivedLogCount: 12345,
			TimeCompleted:    now,
		})

		Expect(spyHTTPClient.postWasCalled).To(Equal(true))
		Expect(spyHTTPClient.url).To(Equal(
			"https://app.datadoghq.com/api/v1/series?api_key=somekey",
		))
		Expect(spyHTTPClient.contentType).To(Equal("application/json;charset=utf-8"))

		actualPayload, _ := ioutil.ReadAll(spyHTTPClient.body)
		expectedPayload := fmt.Sprintf(`{"series":[
			{"metric": "smoke_test.loggregator.msg_count",
			"points": [[%d, %d]],
			"type": "gauge",
			"host": "mycoolhost.cfapps.io",
			"tags": ["firehose-nozzle", "delay:%d"]},
			{"metric": "smoke_test.loggregator.cycles",
			"points": [[%d, %d]],
			"type": "gauge",
			"host": "mycoolhost.cfapps.io",
			"tags": ["firehose-nozzle", "delay:%d"]}]}`,
			now.Unix(), 12345, 1*time.Second,
			now.Unix(), 54321, 1*time.Second,
		)
		Expect(string(actualPayload)).To(MatchJSON(expectedPayload))
	})

	It("returns an error if the HTTP request fails", func() {
		spyHTTPClient := &spyHTTPClient{}
		spyHTTPClient.postResponseReturn = &http.Response{StatusCode: 422}

		r := reliability.NewDataDogReporter(
			"somekey",
			"mycoolhost.cfapps.io",
			spyHTTPClient,
		)

		err := r.Report(&reliability.TestResult{})

		Expect(err).To(HaveOccurred())
	})

	It("returns an error if the client fails", func() {
		spyHTTPClient := &spyHTTPClient{}
		spyHTTPClient.postErrorReturn = errors.New("something failed")

		r := reliability.NewDataDogReporter(
			"somekey",
			"mycoolhost.cfapps.io",
			spyHTTPClient,
		)

		err := r.Report(&reliability.TestResult{})

		Expect(err).To(HaveOccurred())
	})
})

type spyHTTPClient struct {
	url           string
	contentType   string
	postWasCalled bool
	body          io.Reader

	postErrorReturn    error
	postResponseReturn *http.Response
}

func (c *spyHTTPClient) Post(
	url string,
	contentType string,
	body io.Reader,
) (*http.Response, error) {
	c.url = url
	c.contentType = contentType
	c.postWasCalled = true
	c.body = body

	return c.postResponseReturn, c.postErrorReturn
}
