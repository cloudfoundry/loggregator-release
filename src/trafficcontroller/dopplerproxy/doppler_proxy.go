package dopplerproxy

import (
	"fmt"
	"log"
	"mime/multipart"
	"net/http"
	"net/url"
	"plumbing"
	"sync/atomic"
	"trafficcontroller/authorization"
	"trafficcontroller/doppler_endpoint"
	"trafficcontroller/grpcconnector"

	"time"

	"github.com/cloudfoundry/dropsonde/metrics"
	"github.com/cloudfoundry/gosteno"
	"github.com/cloudfoundry/sonde-go/events"
	"github.com/gogo/protobuf/proto"
	"github.com/gorilla/mux"
	"golang.org/x/net/context"
)

const (
	FIREHOSE_ID     = "firehose"
	metricsInterval = time.Second
)

type Proxy struct {
	mux.Router

	logAuthorize   authorization.LogAccessAuthorizer
	adminAuthorize authorization.AdminAccessAuthorizer
	grpcConn       grpcConnector
	cookieDomain   string
	logger         *gosteno.Logger
	numFirehoses   int64
	numAppStreams  int64
	timeout        time.Duration
}

// TODO export this
type grpcConnector interface {
	Subscribe(ctx context.Context, req *plumbing.SubscriptionRequest) (grpcconnector.Receiver, error)
	ContainerMetrics(ctx context.Context, appID string) [][]byte
	RecentLogs(ctx context.Context, appID string) [][]byte
}

func NewDopplerProxy(
	logAuthorize authorization.LogAccessAuthorizer,
	adminAuthorizer authorization.AdminAccessAuthorizer,
	grpcConn grpcConnector,
	cookieDomain string,
	logger *gosteno.Logger,
	timeout time.Duration,
) *Proxy {
	p := &Proxy{
		logAuthorize:   logAuthorize,
		adminAuthorize: adminAuthorizer,
		grpcConn:       grpcConn,
		cookieDomain:   cookieDomain,
		logger:         logger,
		timeout:        timeout,
	}
	r := mux.NewRouter()
	p.Router = *r
	p.HandleFunc("/apps/{appID}/stream", p.stream)
	p.HandleFunc("/apps/{appID}/recentlogs", p.recentlogs)
	p.HandleFunc("/apps/{appID}/containermetrics", p.containermetrics)
	p.HandleFunc("/firehose/{subID}", p.firehose)
	p.HandleFunc("/set-cookie", p.setcookie)

	go p.emitMetrics()

	return p
}

func (p *Proxy) emitMetrics() {
	for range time.Tick(metricsInterval) {
		metrics.SendValue("dopplerProxy.firehoses", float64(atomic.LoadInt64(&p.numFirehoses)), "connections")
		metrics.SendValue("dopplerProxy.appStreams", float64(atomic.LoadInt64(&p.numAppStreams)), "connections")
	}
}

func (p *Proxy) firehose(w http.ResponseWriter, r *http.Request) {
	atomic.AddInt64(&p.numFirehoses, 1)
	defer atomic.AddInt64(&p.numFirehoses, -1)

	subID := mux.Vars(r)["subID"]
	p.serveFirehose(subID, w, r)
}

func (p *Proxy) stream(w http.ResponseWriter, r *http.Request) {
	atomic.AddInt64(&p.numAppStreams, 1)
	defer atomic.AddInt64(&p.numAppStreams, -1)

	p.serveAppLogs("stream", mux.Vars(r)["appID"], w, r)
}

func (p *Proxy) recentlogs(w http.ResponseWriter, r *http.Request) {
	p.serveAppLogs("recentlogs", mux.Vars(r)["appID"], w, r)
	sendLatencyMetric("recentlogs", time.Now())
}

func (p *Proxy) containermetrics(w http.ResponseWriter, r *http.Request) {
	p.serveAppLogs("containermetrics", mux.Vars(r)["appID"], w, r)
	sendLatencyMetric("containermetrics", time.Now())
}

func (p *Proxy) setcookie(w http.ResponseWriter, r *http.Request) {
	p.serveSetCookie(w, r, p.cookieDomain)
}

func (p *Proxy) serveFirehose(firehoseSubscriptionId string, writer http.ResponseWriter, request *http.Request) {
	authToken := getAuthToken(request)

	authorized, err := p.adminAuthorize(authToken, p.logger)
	if !authorized {
		writer.Header().Set("WWW-Authenticate", "Basic")
		writer.WriteHeader(http.StatusUnauthorized)
		fmt.Fprintf(writer, "You are not authorized. %s", err.Error())
		p.logger.Warnf("auth token not authorized to access firehose")
		return
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	client, err := p.grpcConn.Subscribe(ctx, &plumbing.SubscriptionRequest{
		ShardID: firehoseSubscriptionId,
	})
	if err != nil {
		writer.WriteHeader(http.StatusServiceUnavailable)
		log.Println("error occured when subscribing to doppler: %s", err)
		return
	}

	p.serveWS(FIREHOSE_ID, firehoseSubscriptionId, writer, request, client.Recv)
}

// "^/apps/(.*)/(recentlogs|stream|containermetrics)$"
func (p *Proxy) serveAppLogs(requestPath, appID string, writer http.ResponseWriter, request *http.Request) {
	authToken := getAuthToken(request)

	status, err := p.logAuthorize(authToken, appID, p.logger)
	if status != http.StatusOK {
		p.logger.Warndf(map[string]interface{}{
			"status": status,
			"err":    err,
		}, "auth token not authorized to access appID [%s].", appID)
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

	switch requestPath {
	case "recentlogs":
		ctx, _ = context.WithDeadline(ctx, time.Now().Add(p.timeout))
		resp := p.grpcConn.RecentLogs(ctx, appID)
		if err := ctx.Err(); err != nil {
			writer.WriteHeader(http.StatusServiceUnavailable)
			log.Printf("recentlogs request encountered an error: %s", err)
			return
		}
		p.serveMultiPartResponse(writer, resp)
		return
	case "containermetrics":
		ctx, _ = context.WithDeadline(ctx, time.Now().Add(p.timeout))
		resp := deDupe(p.grpcConn.ContainerMetrics(ctx, appID))
		if err := ctx.Err(); err != nil {
			writer.WriteHeader(http.StatusServiceUnavailable)
			log.Printf("containermetrics request encountered an error: %s", err)
			return
		}
		p.serveMultiPartResponse(writer, resp)
		return
	case "stream":
		client, err := p.grpcConn.Subscribe(ctx, &plumbing.SubscriptionRequest{
			Filter: &plumbing.Filter{
				AppID: appID,
			},
		})
		if err != nil {
			writer.WriteHeader(http.StatusServiceUnavailable)
			return
		}

		p.serveWS(requestPath, appID, writer, request, client.Recv)
		return
	}
}

func (p *Proxy) serveWS(endpointType, streamID string, w http.ResponseWriter, r *http.Request, recv func() ([]byte, error)) {
	dopplerEndpoint := doppler_endpoint.NewDopplerEndpoint(endpointType, streamID, false)
	data := make(chan []byte)
	handler := dopplerEndpoint.HProvider(data, p.logger)

	go func() {
		defer close(data)
		timer := time.NewTimer(5 * time.Second)
		timer.Stop()
		for {
			resp, err := recv()
			if err != nil {
				p.logger.Error(err.Error())
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
				metrics.SendValue("dopplerProxy.slowConsumer", 1, "consumer")
				log.Print("Doppler Proxy: Slow Consumer")
				return
			}
		}
	}()
	handler.ServeHTTP(w, r)
}

func (p *Proxy) serveMultiPartResponse(rw http.ResponseWriter, messages [][]byte) {
	mp := multipart.NewWriter(rw)
	defer mp.Close()

	rw.Header().Set("Content-Type", `multipart/x-protobuf; boundary=`+mp.Boundary())

	for _, message := range messages {
		partWriter, err := mp.CreatePart(nil)
		if err != nil {
			p.logger.Infof("http handler: Client went away while serving recent logs")
			return
		}

		partWriter.Write(message)
	}
}

func sendLatencyMetric(metricName string, startTime time.Time) {
	elapsedMillisecond := float64(time.Since(startTime)) / float64(time.Millisecond)
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

func (p *Proxy) serveSetCookie(writer http.ResponseWriter, request *http.Request, cookieDomain string) {
	err := request.ParseForm()
	if err != nil {
		writer.WriteHeader(http.StatusBadRequest)
	}

	cookieName := request.FormValue("CookieName")
	cookieValue := request.FormValue("CookieValue")
	origin := request.Header.Get("Origin")

	http.SetCookie(writer, &http.Cookie{Name: cookieName, Value: cookieValue, Domain: cookieDomain, Secure: true})

	writer.Header().Add("Access-Control-Allow-Credentials", "true")
	writer.Header().Add("Access-Control-Allow-Origin", origin)

	p.logger.Debugf("Proxy: Set cookie name '%s' for origin '%s' on domain '%s'", cookieName, origin, cookieDomain)
}

func deDupe(input [][]byte) [][]byte {
	messages := make(map[int32]*events.Envelope)

	for _, message := range input {
		var envelope events.Envelope
		proto.Unmarshal(message, &envelope)
		cm := envelope.GetContainerMetric()

		oldEnvelope, ok := messages[cm.GetInstanceIndex()]
		if !ok || oldEnvelope.GetTimestamp() < envelope.GetTimestamp() {
			messages[cm.GetInstanceIndex()] = &envelope
		}
	}

	output := make([][]byte, 0, len(messages))

	for _, envelope := range messages {
		bytes, _ := proto.Marshal(envelope)
		output = append(output, bytes)
	}
	return output
}
