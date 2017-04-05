package proxy

import (
	"fmt"
	"log"
	"mime/multipart"
	"net/http"
	"net/url"
	"plumbing"
	"sync/atomic"
	"trafficcontroller/internal/auth"

	"time"

	"github.com/cloudfoundry/dropsonde/metrics"
	"github.com/gorilla/mux"
	"golang.org/x/net/context"
)

const (
	FIREHOSE_ID     = "firehose"
	metricsInterval = time.Second
)

type DopplerProxy struct {
	mux.Router

	logAuthorize   auth.LogAccessAuthorizer
	adminAuthorize auth.AdminAccessAuthorizer
	grpcConn       grpcConnector
	cookieDomain   string
	numFirehoses   int64
	numAppStreams  int64
	timeout        time.Duration
}

// TODO export this
type grpcConnector interface {
	Subscribe(ctx context.Context, req *plumbing.SubscriptionRequest) (func() ([]byte, error), error)
	ContainerMetrics(ctx context.Context, appID string) [][]byte
	RecentLogs(ctx context.Context, appID string) [][]byte
}

func NewDopplerProxy(
	logAuthorize auth.LogAccessAuthorizer,
	adminAuthorizer auth.AdminAccessAuthorizer,
	grpcConn grpcConnector,
	cookieDomain string,
	timeout time.Duration,
) *DopplerProxy {
	p := &DopplerProxy{
		logAuthorize:   logAuthorize,
		adminAuthorize: adminAuthorizer,
		grpcConn:       grpcConn,
		cookieDomain:   cookieDomain,
		timeout:        timeout,
	}
	r := mux.NewRouter()
	p.Router = *r

	p.HandleFunc("/apps/{appID}/stream", p.serveStream)
	p.HandleFunc("/firehose/{subID}", p.serveFirehose)

	p.Handle("/set-cookie", NewSetCookieHandler(p.cookieDomain))

	containerMetricsHandler := NewLogAccessMiddleware(
		p.logAuthorize,
		NewContainerMetricsHandler(p.grpcConn, p.timeout),
	)
	p.Handle("/apps/{appID}/containermetrics", containerMetricsHandler)

	recentLogsHandler := NewLogAccessMiddleware(
		p.logAuthorize,
		NewRecentLogsHandler(p.grpcConn, p.timeout),
	)
	p.Handle("/apps/{appID}/recentlogs", recentLogsHandler)

	go p.emitMetrics()

	return p
}

func (p *DopplerProxy) emitMetrics() {
	for range time.Tick(metricsInterval) {
		// metric-documentation-v1: (dopplerProxy.firehoses) Number of open firehose streams
		metrics.SendValue("dopplerProxy.firehoses", float64(atomic.LoadInt64(&p.numFirehoses)), "connections")
		// metric-documentation-v1: (dopplerProxy.appStreams) Number of open app streams
		metrics.SendValue("dopplerProxy.appStreams", float64(atomic.LoadInt64(&p.numAppStreams)), "connections")
	}
}

func (p *DopplerProxy) serveFirehose(writer http.ResponseWriter, request *http.Request) {
	atomic.AddInt64(&p.numFirehoses, 1)
	defer atomic.AddInt64(&p.numFirehoses, -1)

	subID := mux.Vars(request)["subID"]
	authToken := getAuthToken(request)

	authorized, err := p.adminAuthorize(authToken)
	if !authorized {
		writer.Header().Set("WWW-Authenticate", "Basic")
		writer.WriteHeader(http.StatusUnauthorized)
		fmt.Fprintf(writer, "You are not authorized. %s", err.Error())
		return
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	client, err := p.grpcConn.Subscribe(ctx, &plumbing.SubscriptionRequest{
		ShardID: subID,
	})
	if err != nil {
		writer.WriteHeader(http.StatusServiceUnavailable)
		log.Printf("error occurred when subscribing to doppler: %s", err)
		return
	}

	p.serveWS(FIREHOSE_ID, subID, writer, request, client)
}

// "^/apps/(.*)/(recentlogs|stream|containermetrics)$"
func (p *DopplerProxy) serveStream(writer http.ResponseWriter, request *http.Request) {
	atomic.AddInt64(&p.numAppStreams, 1)
	defer atomic.AddInt64(&p.numAppStreams, -1)

	appID := mux.Vars(request)["appID"]
	authToken := getAuthToken(request)

	status, _ := p.logAuthorize(authToken, appID)
	if status != http.StatusOK {
		switch status {
		case http.StatusUnauthorized:
			writer.WriteHeader(status)
			writer.Header().Set("WWW-Authenticate", "Basic")
		case http.StatusForbidden, http.StatusNotFound:
			status = http.StatusNotFound
		default:
			status = http.StatusInternalServerError
		}

		writer.WriteHeader(status)

		return
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	client, err := p.grpcConn.Subscribe(ctx, &plumbing.SubscriptionRequest{
		Filter: &plumbing.Filter{
			AppID: appID,
		},
	})
	if err != nil {
		writer.WriteHeader(http.StatusServiceUnavailable)
		return
	}

	p.serveWS("stream", appID, writer, request, client)
	return
}

func (p *DopplerProxy) serveWS(endpointType, streamID string, w http.ResponseWriter, r *http.Request, recv func() ([]byte, error)) {
	dopplerEndpoint := NewDopplerEndpoint(endpointType, streamID, false)
	data := make(chan []byte)
	handler := dopplerEndpoint.HProvider(data)

	go func() {
		defer close(data)
		timer := time.NewTimer(5 * time.Second)
		timer.Stop()
		for {
			resp, err := recv()
			if err != nil {
				log.Printf("error receiving from doppler via gRPC %s", err)
				return
			}

			if resp == nil {
				continue
			}

			timer.Reset(5 * time.Second)
			select {
			case data <- resp:
				if !timer.Stop() {
					<-timer.C
				}
			case <-timer.C:
				// metric-documentation-v1: (dopplerProxy.slowConsumer) A slow consumer of the
				// websocket stream
				metrics.SendValue("dopplerProxy.slowConsumer", 1, "consumer")
				log.Print("Doppler Proxy: Slow Consumer")
				return
			}
		}
	}()

	handler.ServeHTTP(w, r)
}

func serveMultiPartResponse(rw http.ResponseWriter, messages [][]byte) {
	mp := multipart.NewWriter(rw)
	defer mp.Close()

	rw.Header().Set("Content-Type", `multipart/x-protobuf; boundary=`+mp.Boundary())

	for _, message := range messages {
		partWriter, err := mp.CreatePart(nil)
		if err != nil {
			log.Printf("http handler: Client went away while serving recent logs")
			return
		}

		partWriter.Write(message)
	}
}

func sendLatencyMetric(metricName string, startTime time.Time) {
	elapsedMillisecond := float64(time.Since(startTime)) / float64(time.Millisecond)
	// metric-documentation-v1: see callers of sendLatencyMetric
	metrics.SendValue(fmt.Sprintf("dopplerProxy.%sLatency", metricName), elapsedMillisecond, "ms")
}

func getAuthToken(req *http.Request) string {
	authToken := req.Header.Get("Authorization")

	if authToken == "" {
		authToken = extractAuthTokenFromCookie(req.Cookies())
	}

	return authToken
}

func extractAuthTokenFromCookie(cookies []*http.Cookie) string {
	for _, cookie := range cookies {
		if cookie.Name == "authorization" {
			value, err := url.QueryUnescape(cookie.Value)
			if err != nil {
				return ""
			}

			return value
		}
	}

	return ""
}
