//go:generate hel

package proxy_test

import (
	"io"
	"io/ioutil"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"time"

	"trafficcontroller/internal/proxy"

	"github.com/cloudfoundry/sonde-go/events"
	"github.com/gogo/protobuf/proto"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("DopplerProxy", func() {
	var (
		auth         LogAuthorizer
		adminAuth    AdminAuthorizer
		dopplerProxy *proxy.DopplerProxy
		recorder     *httptest.ResponseRecorder

		mockGrpcConnector       *mockGrpcConnector
		mockDopplerStreamClient *mockReceiver
	)

	BeforeEach(func() {
		auth = LogAuthorizer{Result: AuthorizerResult{Status: http.StatusOK}}
		adminAuth = AdminAuthorizer{Result: AuthorizerResult{Status: http.StatusOK}}
		mockGrpcConnector = newMockGrpcConnector()

		mockDopplerStreamClient = newMockReceiver()

		mockGrpcConnector.SubscribeOutput.Ret0 <- mockDopplerStreamClient.Recv

		dopplerProxy = proxy.NewDopplerProxy(
			auth.Authorize,
			adminAuth.Authorize,
			mockGrpcConnector,
			"cookieDomain",
			50*time.Millisecond,
		)

		recorder = httptest.NewRecorder()

		fakeMetricSender.Reset()
	})

	JustBeforeEach(func() {
		close(mockGrpcConnector.SubscribeOutput.Ret0)
		close(mockGrpcConnector.SubscribeOutput.Ret1)

		close(mockDopplerStreamClient.RecvOutput.Ret0)
		close(mockDopplerStreamClient.RecvOutput.Ret1)
	})

	Describe("TrafficController emitted request metrics", func() {
		requestAndAssert := func(req *http.Request, metricName string) {
			requestStart := time.Now()
			dopplerProxy.ServeHTTP(recorder, req)
			metric := fakeMetricSender.GetValue(metricName)
			elapsed := float64(time.Since(requestStart)) / float64(time.Millisecond)

			Expect(metric.Unit).To(Equal("ms"))
			Expect(metric.Value).To(BeNumerically("<", elapsed))
		}

		It("emits latency value metric for recentlogs request", func() {
			mockGrpcConnector.RecentLogsOutput.Ret0 <- nil
			req, _ := http.NewRequest("GET", "/apps/appID123/recentlogs", nil)
			requestAndAssert(req, "dopplerProxy.recentlogsLatency")
		})

		It("emits latency value metric for containermetrics request", func() {
			mockGrpcConnector.ContainerMetricsOutput.Ret0 <- nil

			req, _ := http.NewRequest("GET", "/apps/appID123/containermetrics", nil)
			requestAndAssert(req, "dopplerProxy.containermetricsLatency")
		})

		It("does not emit any latency metrics for stream request", func() {
			req, _ := http.NewRequest("GET", "/apps/appID123/stream", nil)
			dopplerProxy.ServeHTTP(recorder, req)
			metric := fakeMetricSender.GetValue("dopplerProxy.streamLatency")

			Expect(metric.Unit).To(BeEmpty())
			Expect(metric.Value).To(BeZero())
		})
	})

	It("returns the requested container metrics", func(done Done) {
		defer close(done)
		req, _ := http.NewRequest("GET", "/apps/abc123/containermetrics", nil)
		req.Header.Add("Authorization", "token")
		now := time.Now()
		_, envBytes1 := buildContainerMetric("abc123", now)
		_, envBytes2 := buildContainerMetric("abc123", now.Add(-5*time.Minute))
		containerResp := [][]byte{
			envBytes1,
			envBytes2,
		}
		mockGrpcConnector.ContainerMetricsOutput.Ret0 <- containerResp

		dopplerProxy.ServeHTTP(recorder, req)

		boundaryRegexp := regexp.MustCompile("boundary=(.*)")
		matches := boundaryRegexp.FindStringSubmatch(recorder.Header().Get("Content-Type"))
		Expect(matches).To(HaveLen(2))
		Expect(matches[1]).NotTo(BeEmpty())
		reader := multipart.NewReader(recorder.Body, matches[1])

		part, err := reader.NextPart()
		Expect(err).ToNot(HaveOccurred())

		partBytes, err := ioutil.ReadAll(part)
		Expect(err).ToNot(HaveOccurred())
		Expect(partBytes).To(Equal(containerResp[0]))
	})

	It("returns the requested recent logs", func() {
		req, _ := http.NewRequest("GET", "/apps/abc123/recentlogs", nil)
		req.Header.Add("Authorization", "token")
		recentLogResp := [][]byte{
			[]byte("log1"),
			[]byte("log2"),
			[]byte("log3"),
		}
		mockGrpcConnector.RecentLogsOutput.Ret0 <- recentLogResp

		dopplerProxy.ServeHTTP(recorder, req)

		boundaryRegexp := regexp.MustCompile("boundary=(.*)")
		matches := boundaryRegexp.FindStringSubmatch(recorder.Header().Get("Content-Type"))
		Expect(matches).To(HaveLen(2))
		Expect(matches[1]).NotTo(BeEmpty())
		reader := multipart.NewReader(recorder.Body, matches[1])

		for _, payload := range recentLogResp {
			part, err := reader.NextPart()
			Expect(err).ToNot(HaveOccurred())

			partBytes, err := ioutil.ReadAll(part)
			Expect(err).ToNot(HaveOccurred())
			Expect(partBytes).To(Equal(payload))
		}
	})

	It("returns the requested recent logs with limit", func() {
		req, _ := http.NewRequest("GET", "/apps/abc123/recentlogs?limit=2", nil)
		req.Header.Add("Authorization", "token")
		recentLogResp := [][]byte{
			[]byte("log1"),
			[]byte("log2"),
			[]byte("log3"),
		}
		mockGrpcConnector.RecentLogsOutput.Ret0 <- recentLogResp

		dopplerProxy.ServeHTTP(recorder, req)

		boundaryRegexp := regexp.MustCompile("boundary=(.*)")
		matches := boundaryRegexp.FindStringSubmatch(recorder.Header().Get("Content-Type"))
		Expect(matches).To(HaveLen(2))
		Expect(matches[1]).NotTo(BeEmpty())
		reader := multipart.NewReader(recorder.Body, matches[1])

		var count int
		for {
			_, err := reader.NextPart()
			if err == io.EOF {
				break
			}
			count++
		}
		Expect(count).To(Equal(2))
	})

	It("ignores limit if it is negative", func() {
		req, _ := http.NewRequest("GET", "/apps/abc123/recentlogs?limit=-2", nil)
		req.Header.Add("Authorization", "token")
		recentLogResp := [][]byte{
			[]byte("log1"),
			[]byte("log2"),
			[]byte("log3"),
		}
		mockGrpcConnector.RecentLogsOutput.Ret0 <- recentLogResp

		dopplerProxy.ServeHTTP(recorder, req)

		boundaryRegexp := regexp.MustCompile("boundary=(.*)")
		matches := boundaryRegexp.FindStringSubmatch(recorder.Header().Get("Content-Type"))
		Expect(matches).To(HaveLen(2))
		Expect(matches[1]).NotTo(BeEmpty())
		reader := multipart.NewReader(recorder.Body, matches[1])

		for _, payload := range recentLogResp {
			part, err := reader.NextPart()
			Expect(err).ToNot(HaveOccurred())

			partBytes, err := ioutil.ReadAll(part)
			Expect(err).ToNot(HaveOccurred())
			Expect(partBytes).To(Equal(payload))
		}
	})

	Context("SetCookie", func() {
		It("returns an OK status with a form", func() {
			req, _ := http.NewRequest("POST", "/set-cookie", strings.NewReader("CookieName=cookie&CookieValue=monster"))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

			dopplerProxy.ServeHTTP(recorder, req)

			Expect(recorder.Code).To(Equal(http.StatusOK))
		})

		It("sets the passed value as a cookie and sets CORS headers", func() {
			req, _ := http.NewRequest("POST", "/set-cookie", strings.NewReader("CookieName=cookie&CookieValue=monster"))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			req.Header.Set("Origin", "fake-origin-string")

			dopplerProxy.ServeHTTP(recorder, req)

			Expect(recorder.Header().Get("Set-Cookie")).To(Equal("cookie=monster; Domain=cookieDomain; Secure"))
			Expect(recorder.Header().Get("Access-Control-Allow-Origin")).To(Equal("fake-origin-string"))
			Expect(recorder.Header().Get("Access-Control-Allow-Credentials")).To(Equal("true"))
		})

		It("returns a bad request if the form does not parse", func() {
			req, _ := http.NewRequest("POST", "/set-cookie", nil)
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

			dopplerProxy.ServeHTTP(recorder, req)

			Expect(recorder.Code).To(Equal(http.StatusBadRequest))
		})
	})
})

var _ = Describe("DefaultHandlerProvider", func() {
	It("returns an HTTP handler for .../recentlogs", func() {
		httpHandler := proxy.NewHttpHandler(make(chan []byte))

		target := proxy.HttpHandlerProvider(make(chan []byte))

		Expect(target).To(BeAssignableToTypeOf(httpHandler))
	})

	It("returns a Websocket handler for .../stream", func() {
		wsHandler := proxy.NewWebsocketHandler(make(chan []byte), time.Minute)

		target := proxy.WebsocketHandlerProvider(make(chan []byte))

		Expect(target).To(BeAssignableToTypeOf(wsHandler))
	})

	It("returns a Websocket handler for anything else", func() {
		wsHandler := proxy.NewWebsocketHandler(make(chan []byte), time.Minute)

		target := proxy.WebsocketHandlerProvider(make(chan []byte))

		Expect(target).To(BeAssignableToTypeOf(wsHandler))
	})
})

func buildContainerMetric(appID string, t time.Time) (*events.Envelope, []byte) {
	envelope := &events.Envelope{
		Origin:    proto.String("doppler"),
		EventType: events.Envelope_ContainerMetric.Enum(),
		Timestamp: proto.Int64(t.UnixNano()),
		ContainerMetric: &events.ContainerMetric{
			ApplicationId: proto.String(appID),
			InstanceIndex: proto.Int32(int32(1)),
			CpuPercentage: proto.Float64(float64(1)),
			MemoryBytes:   proto.Uint64(uint64(1)),
			DiskBytes:     proto.Uint64(uint64(1)),
		},
	}
	data, err := proto.Marshal(envelope)
	Expect(err).ToNot(HaveOccurred())
	return envelope, data
}
