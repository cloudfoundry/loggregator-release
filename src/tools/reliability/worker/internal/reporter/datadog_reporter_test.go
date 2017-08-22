package reporter_test

import (
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"time"
	"tools/reliability/worker/internal/reporter"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
)

var _ = Describe("DataDogReporter", func() {
	It("sends a test result", func() {
		spyHTTPClient := &spyHTTPClient{}
		spyHTTPClient.postResponseReturn = &http.Response{
			StatusCode: 201,
			Body:       ioutil.NopCloser(nil),
		}
		r := reporter.NewDataDogReporter(
			"somekey",
			"mycoolhost.cfapps.io",
			"sweet-instance-id",
			spyHTTPClient,
		)

		now := time.Unix(0, 20000000000)
		r.Report(&reporter.TestResult{
			Delay:            1 * time.Second,
			Cycles:           54321,
			ReceivedLogCount: 12345,
			TestStartTime:    now,
		})

		Expect(spyHTTPClient.postWasCalled).To(Equal(true))
		Expect(spyHTTPClient.url).To(Equal(
			"https://app.datadoghq.com/api/v1/series?api_key=somekey",
		))
		Expect(spyHTTPClient.contentType).To(Equal("application/json;charset=utf-8"))

		actualPayload, _ := ioutil.ReadAll(spyHTTPClient.body)
		expectedPayload := fmt.Sprintf(`
			{
				"series":[
					{
						"metric": "smoke_test.loggregator.msg_count",
						"points": [[%d, %d]],
						"type": "gauge",
						"host": "mycoolhost.cfapps.io",
						"tags": ["firehose-nozzle", "delay:%d", "instance_index:sweet-instance-id"]},
					{
						"metric": "smoke_test.loggregator.cycles",
						"points": [[%d, %d]],
						"type": "gauge",
						"host": "mycoolhost.cfapps.io",
						"tags": ["firehose-nozzle", "delay:%d", "instance_index:sweet-instance-id"]
					}
				]
			}
			`,
			20, 12345, 1*time.Second,
			20, 54321, 1*time.Second,
		)
		Expect(string(actualPayload)).To(MatchJSON(expectedPayload))
	})

	It("returns an error if the HTTP request fails", func() {
		spyHTTPClient := &spyHTTPClient{}
		spyHTTPClient.postResponseReturn = &http.Response{
			StatusCode: 422,
			Body:       ioutil.NopCloser(nil),
		}

		r := reporter.NewDataDogReporter(
			"somekey",
			"mycoolhost.cfapps.io",
			"sweet-instance-id",
			spyHTTPClient,
		)

		err := r.Report(&reporter.TestResult{})

		Expect(err).To(HaveOccurred())
	})

	It("closes the response body", func() {
		spyReadCloser := &spyIOReadCloser{}
		spyHTTPClient := &spyHTTPClient{}
		spyHTTPClient.postResponseReturn = &http.Response{
			StatusCode: 422,
			Body:       spyReadCloser,
		}

		r := reporter.NewDataDogReporter(
			"somekey",
			"mycoolhost.cfapps.io",
			"sweet-instance-id",
			spyHTTPClient,
		)
		_ = r.Report(&reporter.TestResult{})

		Expect(spyReadCloser.wasClosed).To(BeTrue())
	})

	DescribeTable("status codes", func(status int, expectedErr error) {
		spyHTTPClient := &spyHTTPClient{}
		spyHTTPClient.postResponseReturn = &http.Response{
			StatusCode: status,
			Body:       ioutil.NopCloser(nil),
		}

		r := reporter.NewDataDogReporter(
			"somekey",
			"mycoolhost.cfapps.io",
			"sweet-instance-id",
			spyHTTPClient,
		)

		err := r.Report(&reporter.TestResult{})
		if expectedErr == nil {
			Expect(err).To(BeNil())
		} else {
			Expect(err).To(Equal(expectedErr))
		}
	},
		Entry("Created", http.StatusCreated, nil),
		Entry("Accepted", http.StatusAccepted, nil),
		Entry("OK", http.StatusOK, nil),
		Entry("Internal Server Error", http.StatusInternalServerError, errors.New("status code was 500")),
	)

	It("returns an error if the client fails", func() {
		spyHTTPClient := &spyHTTPClient{}
		spyHTTPClient.postErrorReturn = errors.New("something failed")

		r := reporter.NewDataDogReporter(
			"somekey",
			"mycoolhost.cfapps.io",
			"sweet-instance-id",
			spyHTTPClient,
		)

		err := r.Report(&reporter.TestResult{})

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

type spyIOReadCloser struct {
	io.Reader
	wasClosed bool
}

func (c *spyIOReadCloser) Close() error {
	c.wasClosed = true
	return nil
}
