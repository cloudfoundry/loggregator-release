package fake_doppler

import (
	"net"
	"net/http"
	"plumbing"
	"sync"
	"time"

	"golang.org/x/net/context"

	"github.com/cloudfoundry/loggregatorlib/loggertesthelper"
	"github.com/cloudfoundry/loggregatorlib/server/handlers"
	"github.com/gorilla/websocket"
	"google.golang.org/grpc"
)

type FakeDoppler struct {
	ApiEndpoint                string
	GrpcEndpoint               string
	connectionListener         net.Listener
	grpcListener               net.Listener
	websocket                  *websocket.Conn
	sendMessageChan            chan []byte
	grpcOut                    chan []byte
	TrafficControllerConnected chan *http.Request
	SubscriptionRequests       chan *plumbing.SubscriptionRequest
	ContainerMetricsRequests   chan *plumbing.ContainerMetricsRequest
	RecentLogsRequests         chan *plumbing.RecentLogsRequest
	SubscribeServers           chan plumbing.Doppler_SubscribeServer
	connectionPresent          bool
	done                       chan struct{}
	sync.RWMutex
}

func New() *FakeDoppler {
	return &FakeDoppler{
		ApiEndpoint:                "127.0.0.1:1235",
		GrpcEndpoint:               "127.0.0.1:1236",
		TrafficControllerConnected: make(chan *http.Request, 1),
		sendMessageChan:            make(chan []byte, 100),
		grpcOut:                    make(chan []byte, 100),
		SubscriptionRequests:       make(chan *plumbing.SubscriptionRequest, 100),
		ContainerMetricsRequests:   make(chan *plumbing.ContainerMetricsRequest, 100),
		RecentLogsRequests:         make(chan *plumbing.RecentLogsRequest, 100),
		SubscribeServers:           make(chan plumbing.Doppler_SubscribeServer, 100),
		done:                       make(chan struct{}),
	}
}

func (fakeDoppler *FakeDoppler) Start() error {
	defer close(fakeDoppler.done)

	var err error
	fakeDoppler.Lock()
	fakeDoppler.grpcListener, err = net.Listen("tcp", fakeDoppler.GrpcEndpoint)
	fakeDoppler.Unlock()
	if err != nil {
		return err
	}

	grpcServer := grpc.NewServer()
	plumbing.RegisterDopplerServer(grpcServer, fakeDoppler)

	go grpcServer.Serve(fakeDoppler.grpcListener)

	fakeDoppler.Lock()
	fakeDoppler.connectionListener, err = net.Listen("tcp", fakeDoppler.ApiEndpoint)
	fakeDoppler.Unlock()
	if err != nil {
		return err

	}
	s := &http.Server{Addr: fakeDoppler.ApiEndpoint, Handler: fakeDoppler}

	err = s.Serve(fakeDoppler.connectionListener)
	return err
}

func (fakeDoppler *FakeDoppler) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	select {
	case fakeDoppler.TrafficControllerConnected <- request:
	default:
	}

	fakeDoppler.Lock()
	fakeDoppler.connectionPresent = true
	fakeDoppler.Unlock()

	handlers.NewWebsocketHandler(fakeDoppler.sendMessageChan, time.Millisecond*100, loggertesthelper.Logger()).ServeHTTP(writer, request)

	fakeDoppler.Lock()
	fakeDoppler.connectionPresent = false
	fakeDoppler.Unlock()
}

func (fakeDoppler *FakeDoppler) Stop() {
	fakeDoppler.Lock()
	if fakeDoppler.grpcListener != nil {
		fakeDoppler.grpcListener.Close()
	}
	if fakeDoppler.connectionListener != nil {
		fakeDoppler.connectionListener.Close()
	}
	fakeDoppler.Unlock()
	<-fakeDoppler.done
}

func (fakeDoppler *FakeDoppler) ConnectionPresent() bool {
	fakeDoppler.Lock()
	defer fakeDoppler.Unlock()

	return fakeDoppler.connectionPresent
}

func (fakeDoppler *FakeDoppler) SendLogMessage(messageBody []byte) {
	fakeDoppler.grpcOut <- messageBody
}

func (fakeDoppler *FakeDoppler) SendWSLogMessage(messageBody []byte) {
	fakeDoppler.sendMessageChan <- messageBody
}

func (fakeDoppler *FakeDoppler) CloseLogMessageStream() {
	close(fakeDoppler.grpcOut)
	close(fakeDoppler.sendMessageChan)
}

func (fakeDoppler *FakeDoppler) Subscribe(request *plumbing.SubscriptionRequest, server plumbing.Doppler_SubscribeServer) error {
	fakeDoppler.SubscriptionRequests <- request
	fakeDoppler.SubscribeServers <- server
	for msg := range fakeDoppler.grpcOut {
		err := server.Send(&plumbing.Response{
			Payload: msg,
		})
		if err != nil {
			return err
		}
	}
	return nil
}

func (fakeDoppler *FakeDoppler) ContainerMetrics(ctx context.Context, request *plumbing.ContainerMetricsRequest) (*plumbing.ContainerMetricsResponse, error) {
	fakeDoppler.ContainerMetricsRequests <- request
	resp := new(plumbing.ContainerMetricsResponse)
	for msg := range fakeDoppler.grpcOut {
		resp.Payload = append(resp.Payload, msg)
	}

	return resp, nil
}

func (fakeDoppler *FakeDoppler) RecentLogs(ctx context.Context, request *plumbing.RecentLogsRequest) (*plumbing.RecentLogsResponse, error) {
	fakeDoppler.RecentLogsRequests <- request
	resp := new(plumbing.RecentLogsResponse)
	for msg := range fakeDoppler.grpcOut {
		resp.Payload = append(resp.Payload, msg)
	}

	return resp, nil
}
