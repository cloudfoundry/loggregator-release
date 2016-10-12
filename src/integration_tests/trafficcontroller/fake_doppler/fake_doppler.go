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
	StreamRequests             chan *plumbing.StreamRequest
	FirehoseRequests           chan *plumbing.FirehoseRequest
	ContainerMetricsRequests   chan *plumbing.ContainerMetricsRequest
	StreamServers              chan plumbing.Doppler_StreamServer
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
		StreamRequests:             make(chan *plumbing.StreamRequest, 100),
		FirehoseRequests:           make(chan *plumbing.FirehoseRequest, 100),
		ContainerMetricsRequests:   make(chan *plumbing.ContainerMetricsRequest, 100),
		StreamServers:              make(chan plumbing.Doppler_StreamServer, 100),
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

func (fakeDoppler *FakeDoppler) Stream(request *plumbing.StreamRequest, server plumbing.Doppler_StreamServer) error {
	fakeDoppler.StreamRequests <- request
	fakeDoppler.StreamServers <- server
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

func (fakeDoppler *FakeDoppler) Firehose(request *plumbing.FirehoseRequest, server plumbing.Doppler_FirehoseServer) error {
	fakeDoppler.FirehoseRequests <- request
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
