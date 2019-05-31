package proxy_test

import (
	"context"
	"errors"
	"io/ioutil"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"sync"
	"time"

	"code.cloudfoundry.org/go-loggregator/rpc/loggregator_v2"
	logcache "code.cloudfoundry.org/log-cache/pkg/client"
	"code.cloudfoundry.org/loggregator/trafficcontroller/internal/proxy"
	"github.com/cloudfoundry/sonde-go/events"
	"github.com/gogo/protobuf/proto"

	"code.cloudfoundry.org/loggregator/metricemitter/testhelper"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Recent Logs Handler", func() {
	var (
		recentLogsHandler *proxy.RecentLogsHandler
		recorder          *httptest.ResponseRecorder
		logCacheClient    *fakeLogCacheClient
	)

	BeforeEach(func() {
		logCacheClient = newFakeLogCacheClient()
		logCacheClient.envelopes = []*loggregator_v2.Envelope{
			buildV2Gauge("8de7d390-9044-41ff-ab76-432299923511", "1", 2, 3),
			buildV2Log("log1"),
			buildV2Log("log2"),
			buildV2Log("log3"),
		}

		recentLogsHandler = proxy.NewRecentLogsHandler(
			logCacheClient,
			200*time.Millisecond,
			testhelper.NewMetricClient(),
			true,
		)

		recorder = httptest.NewRecorder()
	})

	It("returns the requested recent logs", func() {
		req, _ := http.NewRequest("GET", "/apps/8de7d390-9044-41ff-ab76-432299923511/recentlogs", nil)
		req.Header.Add("Authorization", "token")
		recentLogs := [][]byte{
			[]byte("log1"),
			[]byte("log2"),
			[]byte("log3"),
		}

		recentLogsHandler.ServeHTTP(recorder, req)

		boundaryRegexp := regexp.MustCompile("boundary=(.*)")
		matches := boundaryRegexp.FindStringSubmatch(recorder.Header().Get("Content-Type"))
		Expect(matches).To(HaveLen(2))
		Expect(matches[1]).NotTo(BeEmpty())
		reader := multipart.NewReader(recorder.Body, matches[1])

		for _, logMessage := range recentLogs {
			part, err := reader.NextPart()
			Expect(err).ToNot(HaveOccurred())

			partBytes, err := ioutil.ReadAll(part)
			Expect(err).ToNot(HaveOccurred())

			var logEnvelope events.Envelope
			err = proto.Unmarshal(partBytes, &logEnvelope)
			Expect(err).ToNot(HaveOccurred())

			Expect(logEnvelope.GetLogMessage().GetMessage()).To(Equal(logMessage))
		}
	})

	It("returns the requested recent logs with limit", func() {
		req, _ := http.NewRequest("GET", "/apps/8de7d390-9044-41ff-ab76-432299923511/recentlogs?limit=2", nil)
		req.Header.Add("Authorization", "token")

		recentLogsHandler.ServeHTTP(recorder, req)

		Expect(logCacheClient.queryParams.Get("limit")).To(Equal("2"))
	})

	It("sets the limit to a 1000 by default", func() {
		req, _ := http.NewRequest("GET", "/apps/8de7d390-9044-41ff-ab76-432299923511/recentlogs", nil)
		req.Header.Add("Authorization", "token")

		recentLogsHandler.ServeHTTP(recorder, req)

		Expect(logCacheClient.queryParams.Get("limit")).To(Equal("1000"))
	})

	It("sets the limit to a 1000 when requested limit is a negative number", func() {
		req, _ := http.NewRequest("GET", "/apps/8de7d390-9044-41ff-ab76-432299923511/recentlogs?limit=-10", nil)
		req.Header.Add("Authorization", "token")

		recentLogsHandler.ServeHTTP(recorder, req)

		Expect(logCacheClient.queryParams.Get("limit")).To(Equal("1000"))
	})

	It("tries to retrieve logs and backs off until successful", func() {
		logCacheClient.err <- errors.New("received message larger than max")
		logCacheClient.err <- nil

		req, _ := http.NewRequest("GET", "/apps/8de7d390-9044-41ff-ab76-432299923511/recentlogs", nil)
		req.Header.Add("Authorization", "token")

		recentLogsHandler.ServeHTTP(recorder, req)

		Eventually(func() string {
			return logCacheClient.queryParams.Get("limit")
		}, 5).Should(Equal("750"))
	})

	It("requests envelopes in descending order", func() {
		req, _ := http.NewRequest("GET", "/apps/8de7d390-9044-41ff-ab76-432299923511/recentlogs", nil)
		req.Header.Add("Authorization", "token")

		recentLogsHandler.ServeHTTP(recorder, req)

		Expect(logCacheClient.queryParams.Get("descending")).To(Equal("true"))
	})

	It("requests envelopes of type LOG", func() {
		req, _ := http.NewRequest("GET", "/apps/8de7d390-9044-41ff-ab76-432299923511/recentlogs", nil)
		req.Header.Add("Authorization", "token")

		recentLogsHandler.ServeHTTP(recorder, req)

		Expect(logCacheClient.queryParams.Get("envelope_types")).To(Equal("LOG"))
	})

	It("returns a helpful error message", func() {
		logCacheClient.err <- errors.New("It failed")
		req, _ := http.NewRequest("GET", "/apps/8de7d390-9044-41ff-ab76-432299923511/recentlogs", nil)
		req.Header.Add("Authorization", "token")

		recentLogsHandler.ServeHTTP(recorder, req)
		resp := recorder.Result()
		body, _ := ioutil.ReadAll(resp.Body)

		Expect(resp.StatusCode).To(Equal(http.StatusInternalServerError))
		Expect(string(body)).To(Equal("It failed"))
	})

	It("returns a helpful log message when LogCache is disabled", func() {
		spyMetricClient := testhelper.NewMetricClient()

		recentLogsHandler = proxy.NewRecentLogsHandler(
			logCacheClient,
			200*time.Millisecond,
			spyMetricClient,
			false,
		)

		req, _ := http.NewRequest("GET", "/apps/8de7d390-9044-41ff-ab76-432299923511/recentlogs", nil)
		req.Header.Add("Authorization", "token")

		recentLogsHandler.ServeHTTP(recorder, req)

		boundaryRegexp := regexp.MustCompile("boundary=(.*)")
		matches := boundaryRegexp.FindStringSubmatch(recorder.Header().Get("Content-Type"))
		Expect(matches).To(HaveLen(2))
		Expect(matches[1]).NotTo(BeEmpty())
		reader := multipart.NewReader(recorder.Body, matches[1])

		part, err := reader.NextPart()
		Expect(err).ToNot(HaveOccurred())

		partBytes, err := ioutil.ReadAll(part)
		Expect(err).ToNot(HaveOccurred())

		var logEnvelope events.Envelope
		err = proto.Unmarshal(partBytes, &logEnvelope)
		Expect(err).ToNot(HaveOccurred())

		Expect(string(logEnvelope.GetLogMessage().GetMessage())).To(Equal("recent log endpoint requires a log cache. please talk to you operator"))
		Expect(logEnvelope.GetLogMessage().GetMessageType()).To(Equal(events.LogMessage_ERR))
		Expect(logEnvelope.GetLogMessage().GetSourceType()).To(Equal("Loggregator"))
	})

	It("increments a metric when calls to log cache fail", func() {
		spyMetricClient := testhelper.NewMetricClient()
		logCacheClient.err <- errors.New("Failed to read from Log Cache")

		recentLogsHandler = proxy.NewRecentLogsHandler(
			logCacheClient,
			200*time.Millisecond,
			spyMetricClient,
			true,
		)

		req, _ := http.NewRequest("GET", "/apps/8de7d390-9044-41ff-ab76-432299923511/recentlogs", nil)
		req.Header.Add("Authorization", "token")

		recentLogsHandler.ServeHTTP(recorder, req)

		Expect(spyMetricClient.GetDelta("doppler_proxy.log_cache_failure")).To(Equal(uint64(1)))
	})
})

type fakeLogCacheClient struct {
	mu    sync.Mutex
	block bool

	// Inputs
	ctx         context.Context
	sourceID    string
	start       time.Time
	opts        []logcache.ReadOption
	queryParams url.Values

	// Outputs
	envelopes []*loggregator_v2.Envelope
	err       chan error
}

func newFakeLogCacheClient() *fakeLogCacheClient {
	return &fakeLogCacheClient{
		queryParams: make(map[string][]string),
		err:         make(chan error, 100),
	}
}

func (f *fakeLogCacheClient) Read(
	ctx context.Context,
	sourceID string,
	start time.Time,
	opts ...logcache.ReadOption,
) ([]*loggregator_v2.Envelope, error) {
	f.mu.Lock()
	f.ctx = ctx
	f.sourceID = sourceID
	f.start = start
	f.opts = opts

	for _, o := range opts {
		o(nil, f.queryParams)
	}
	f.mu.Unlock()

	if f.block {
		var block chan int
		<-block
	}

	var err error
	if len(f.err) > 0 {
		err = <-f.err
	}

	return f.envelopes, err
}

func buildV2Log(msg string) *loggregator_v2.Envelope {
	return &loggregator_v2.Envelope{
		Message: &loggregator_v2.Envelope_Log{
			Log: &loggregator_v2.Log{
				Payload: []byte(msg),
			},
		},
	}
}

func buildV2Gauge(sid, instanceId string, cpu float64, t int64) *loggregator_v2.Envelope {
	return &loggregator_v2.Envelope{
		Timestamp:  t,
		SourceId:   sid,
		InstanceId: instanceId,
		Message: &loggregator_v2.Envelope_Gauge{
			Gauge: &loggregator_v2.Gauge{
				Metrics: map[string]*loggregator_v2.GaugeValue{
					"cpu": {
						Unit:  "percentage",
						Value: cpu,
					},
					"memory": {
						Unit:  "bytes",
						Value: 13,
					},
					"disk": {
						Unit:  "bytes",
						Value: 15,
					},
					"memory_quota": {
						Unit:  "bytes",
						Value: 17,
					},
					"disk_quota": {
						Unit:  "bytes",
						Value: 19,
					},
				},
			},
		},
	}
}
