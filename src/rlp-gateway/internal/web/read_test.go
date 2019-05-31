package web_test

import (
	"bufio"
	"context"
	"errors"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"sync"
	"time"

	"code.cloudfoundry.org/go-loggregator/rpc/loggregator_v2"
	"code.cloudfoundry.org/loggregator/rlp-gateway/internal/web"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Read", func() {
	var (
		server *httptest.Server
		lp     *stubLogsProvider
		ctx    context.Context
		cancel func()
	)

	BeforeEach(func() {
		lp = newStubLogsProvider()
		lp._batchResponse = &loggregator_v2.EnvelopeBatch{
			Batch: []*loggregator_v2.Envelope{
				{
					SourceId: "source-id-a",
				},
				{
					SourceId: "source-id-b",
				},
			},
		}
		server = httptest.NewServer(web.ReadHandler(lp, 100*time.Millisecond, time.Minute))
		ctx, cancel = context.WithCancel(context.Background())
	})

	AfterEach(func() {
		cancel()
		server.CloseClientConnections()
		server.Close()
	})

	It("reads from the logs provider and sends SSE to the client", func() {
		req, err := http.NewRequest(http.MethodGet, server.URL+"/v2/read?log", nil)
		Expect(err).ToNot(HaveOccurred())

		req = req.WithContext(ctx)

		resp, err := server.Client().Do(req)
		Expect(err).ToNot(HaveOccurred())

		Eventually(lp.requests).Should(HaveLen(1))

		Expect(lp.requests()[0]).To(Equal(&loggregator_v2.EgressBatchRequest{
			Selectors: []*loggregator_v2.Selector{
				{
					Message: &loggregator_v2.Selector_Log{
						Log: &loggregator_v2.LogSelector{},
					},
				},
			},
			UsePreferredTags: true,
		}))

		Expect(resp.StatusCode).To(Equal(http.StatusOK))
		Expect(resp.Header.Get("Content-Type")).To(Equal("text/event-stream"))
		Expect(resp.Header.Get("Cache-Control")).To(Equal("no-cache"))
		Expect(resp.Header.Get("Connection")).To(Equal("keep-alive"))

		buf := bufio.NewReader(resp.Body)

		line, err := buf.ReadBytes('\n')
		Expect(err).ToNot(HaveOccurred())
		Expect(string(line)).To(HavePrefix("data: "))
		Expect(string(line)).To(HaveSuffix("\n"))
		Expect(string(line[6:])).To(MatchJSON(`{
			"batch": [
				{
					"source_id":"source-id-a",
					"timestamp": "0",
					"instance_id": "",
					"tags": {},
					"deprecated_tags": {}
				},
				{
					"source_id":"source-id-b",
					"timestamp": "0",
					"instance_id": "",
					"tags": {},
					"deprecated_tags": {}
				}
			]
		}`))

		// Read 1 empty new lines
		_, err = buf.ReadBytes('\n')
		Expect(err).ToNot(HaveOccurred())

		line, err = buf.ReadBytes('\n')
		Expect(err).ToNot(HaveOccurred())
		Expect(string(line)).To(HavePrefix("data: "))
		Expect(string(line)).To(HaveSuffix("\n"))
		Expect(string(line[6:])).To(MatchJSON(`{
			"batch": [
				{
					"source_id":"source-id-a",
					"timestamp": "0",
					"instance_id": "",
					"tags": {},
					"deprecated_tags": {}
				},
				{
					"source_id":"source-id-b",
					"timestamp": "0",
					"instance_id": "",
					"tags": {},
					"deprecated_tags": {}
				}
			]
		}`))
	})

	It("emits hearbeats for an empty data stream", func() {
		lp.block = true

		req, err := http.NewRequest(http.MethodGet, server.URL+"/v2/read?log", nil)
		Expect(err).ToNot(HaveOccurred())

		req = req.WithContext(ctx)

		resp, err := server.Client().Do(req)
		Expect(err).ToNot(HaveOccurred())

		buf := bufio.NewReader(resp.Body)

		line, err := buf.ReadBytes('\n')
		Expect(err).ToNot(HaveOccurred())
		Expect(string(line)).To(Equal("event: heartbeat\n"))

		line, err = buf.ReadBytes('\n')
		Expect(err).ToNot(HaveOccurred())
		Expect(string(line)).To(MatchRegexp(`data: \d+`))
	})

	It("contains zero values for gauge metrics", func() {
		lp._batchResponse = &loggregator_v2.EnvelopeBatch{
			Batch: []*loggregator_v2.Envelope{
				{
					SourceId: "source-id-a",
					Message: &loggregator_v2.Envelope_Gauge{
						Gauge: &loggregator_v2.Gauge{
							Metrics: map[string]*loggregator_v2.GaugeValue{
								"cpu": {
									Unit:  "percent",
									Value: 0.0,
								},
							},
						},
					},
				},
			},
		}

		req, err := http.NewRequest(http.MethodGet, server.URL+"/v2/read?gauge", nil)
		Expect(err).ToNot(HaveOccurred())

		req = req.WithContext(ctx)

		resp, err := server.Client().Do(req)
		Expect(err).ToNot(HaveOccurred())

		Eventually(lp.requests).Should(HaveLen(1))

		Expect(lp.requests()[0]).To(Equal(&loggregator_v2.EgressBatchRequest{
			Selectors: []*loggregator_v2.Selector{
				{
					Message: &loggregator_v2.Selector_Gauge{
						Gauge: &loggregator_v2.GaugeSelector{},
					},
				},
			},
			UsePreferredTags: true,
		}))

		Expect(resp.StatusCode).To(Equal(http.StatusOK))
		Expect(resp.Header.Get("Content-Type")).To(Equal("text/event-stream"))
		Expect(resp.Header.Get("Cache-Control")).To(Equal("no-cache"))
		Expect(resp.Header.Get("Connection")).To(Equal("keep-alive"))

		buf := bufio.NewReader(resp.Body)

		line, err := buf.ReadBytes('\n')
		Expect(err).ToNot(HaveOccurred())
		Expect(string(line)).To(HavePrefix("data: "))
		Expect(string(line)).To(HaveSuffix("\n"))
		Expect(string(line[6:])).To(MatchJSON(`{
			"batch": [
				{
					"timestamp": "0",
					"instance_id": "",
					"tags": {},
					"deprecated_tags": {},
					"source_id":"source-id-a",
					"gauge": {
						"metrics": {
							"cpu": {
								"unit": "percent",
								"value": 0
							}
						}
					}
				}
			]
		}`))
	})

	It("adds the shard ID to the egress request", func() {
		req, err := http.NewRequest(
			http.MethodGet,
			server.URL+"/v2/read?log&shard_id=my-shard-id",
			nil,
		)
		Expect(err).ToNot(HaveOccurred())

		req = req.WithContext(ctx)

		_, err = server.Client().Do(req)
		Expect(err).ToNot(HaveOccurred())

		Eventually(lp.requests).Should(HaveLen(1))

		Expect(lp.requests()[0]).To(Equal(&loggregator_v2.EgressBatchRequest{
			ShardId: "my-shard-id",
			Selectors: []*loggregator_v2.Selector{
				{
					Message: &loggregator_v2.Selector_Log{
						Log: &loggregator_v2.LogSelector{},
					},
				},
			},
			UsePreferredTags: true,
		}))
	})

	It("adds the deterministic name to the egress request", func() {
		req, err := http.NewRequest(
			http.MethodGet,
			server.URL+"/v2/read?log&deterministic_name=some-deterministic-name",
			nil,
		)
		Expect(err).ToNot(HaveOccurred())

		req = req.WithContext(ctx)

		_, err = server.Client().Do(req)
		Expect(err).ToNot(HaveOccurred())

		Eventually(lp.requests).Should(HaveLen(1))

		Expect(lp.requests()[0]).To(Equal(&loggregator_v2.EgressBatchRequest{
			DeterministicName: "some-deterministic-name",
			Selectors: []*loggregator_v2.Selector{
				{
					Message: &loggregator_v2.Selector_Log{
						Log: &loggregator_v2.LogSelector{},
					},
				},
			},
			UsePreferredTags: true,
		}))
	})

	It("should warn the client when it's about to close the connection", func() {
		lp := newStubLogsProvider()

		server := httptest.NewServer(web.ReadHandler(lp, 5*time.Minute, 5*time.Millisecond))
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		req, err := http.NewRequest(http.MethodGet, server.URL+"/v2/read?log", nil)
		Expect(err).ToNot(HaveOccurred())

		req = req.WithContext(ctx)

		resp, err := server.Client().Do(req)
		Expect(err).ToNot(HaveOccurred())

		Eventually(lp.requests).Should(HaveLen(1))

		buf := bufio.NewReader(resp.Body)

		line, err := buf.ReadBytes('\n')
		Expect(err).ToNot(HaveOccurred())
		Expect(string(line)).To(Equal("event: closing\n"))

		line, err = buf.ReadBytes('\n')
		Expect(err).ToNot(HaveOccurred())
		Expect(string(line)).To(Equal("data: closing due to stream timeout\n"))

		Eventually(func() error {
			_, err := buf.ReadBytes('\n')
			return err
		}).Should(Equal(io.EOF))
	})

	It("closes the SSE stream if the envelope stream returns any error", func() {
		lp._batchResponse = nil
		lp._errorResponse = errors.New("an error")

		req, err := http.NewRequest(http.MethodGet, server.URL+"/v2/read?log", nil)
		Expect(err).ToNot(HaveOccurred())

		req = req.WithContext(ctx)

		resp, err := server.Client().Do(req)
		Expect(err).ToNot(HaveOccurred())

		buf := bufio.NewReader(resp.Body)
		Eventually(func() error {
			_, err := buf.ReadBytes('\n')
			return err
		}).Should(Equal(io.EOF))
	})

	It("returns a bad request if no selectors are provided in url", func() {
		req, err := http.NewRequest(http.MethodGet, server.URL+"/v2/read", nil)
		Expect(err).ToNot(HaveOccurred())

		resp, err := server.Client().Do(req.WithContext(ctx))
		Expect(err).ToNot(HaveOccurred())

		body, err := ioutil.ReadAll(resp.Body)
		Expect(err).ToNot(HaveOccurred())

		Expect(resp.StatusCode).To(Equal(http.StatusBadRequest))
		Expect(body).To(MatchJSON(`{
			"error": "empty_query",
			"message": "query cannot be empty"
		}`))
	})

	It("returns 405, method not allowed", func() {
		req, err := http.NewRequest(http.MethodPost, server.URL+"/v2/read?log", nil)
		Expect(err).ToNot(HaveOccurred())

		resp, err := server.Client().Do(req.WithContext(ctx))
		Expect(err).ToNot(HaveOccurred())

		body, err := ioutil.ReadAll(resp.Body)
		Expect(err).ToNot(HaveOccurred())

		Expect(resp.StatusCode).To(Equal(http.StatusMethodNotAllowed))
		Expect(body).To(MatchJSON(`{
			"error": "method_not_allowed",
			"message": "method not allowed"
		}`))
	})

	It("return 500, internal server error when streaming is not supported", func() {
		req, err := http.NewRequest(http.MethodGet, server.URL+"/v2/read?log", nil)
		Expect(err).ToNot(HaveOccurred())

		w := &nonFlusherWriter{httptest.NewRecorder()}
		web.ReadHandler(lp, time.Minute, time.Minute)(w, req)

		Expect(w.rw.Code).To(Equal(http.StatusInternalServerError))
		Expect(w.rw.Body).To(MatchJSON(`{
			"error": "streaming_unsupported",
			"message": "request does not support streaming"
		}`))
	})
})

type stubLogsProvider struct {
	mu             sync.Mutex
	_requests      []*loggregator_v2.EgressBatchRequest
	_batchResponse *loggregator_v2.EnvelopeBatch
	_errorResponse error
	block          bool
}

func newStubLogsProvider() *stubLogsProvider {
	return &stubLogsProvider{}
}

func (s *stubLogsProvider) Stream(ctx context.Context, req *loggregator_v2.EgressBatchRequest) web.Receiver {
	s.mu.Lock()
	defer s.mu.Unlock()
	s._requests = append(s._requests, req)

	return func() (*loggregator_v2.EnvelopeBatch, error) {
		if s.block {
			var block chan int
			<-block
		}

		return s._batchResponse, s._errorResponse
	}
}

func (s *stubLogsProvider) requests() []*loggregator_v2.EgressBatchRequest {
	s.mu.Lock()
	defer s.mu.Unlock()

	result := make([]*loggregator_v2.EgressBatchRequest, len(s._requests))
	copy(result, s._requests)

	return result
}

// nonFlusherWriter is a wrapper around the httptest.ResponseRecorder that
// excludes the Flush() function.
type nonFlusherWriter struct {
	rw *httptest.ResponseRecorder
}

func (w *nonFlusherWriter) Header() http.Header {
	return w.rw.Header()
}

func (w *nonFlusherWriter) Write(d []byte) (int, error) {
	return w.rw.Write(d)
}

func (w *nonFlusherWriter) WriteHeader(statusCode int) {
	w.rw.WriteHeader(statusCode)
}
