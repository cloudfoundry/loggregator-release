package trafficcontroller_test

import (
	"net"
	"sync"

	"code.cloudfoundry.org/loggregator/plumbing"

	"golang.org/x/net/context"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

type FakeDoppler struct {
	GrpcEndpoint             string
	grpcListener             net.Listener
	grpcOut                  chan []byte
	SubscriptionRequests     chan *plumbing.SubscriptionRequest
	ContainerMetricsRequests chan *plumbing.ContainerMetricsRequest
	RecentLogsRequests       chan *plumbing.RecentLogsRequest
	SubscribeServers         chan plumbing.Doppler_SubscribeServer
	done                     chan struct{}
	sync.RWMutex
}

func NewFakeDoppler() *FakeDoppler {
	return &FakeDoppler{
		GrpcEndpoint:             "127.0.0.1:1236",
		grpcOut:                  make(chan []byte, 100),
		SubscriptionRequests:     make(chan *plumbing.SubscriptionRequest, 100),
		ContainerMetricsRequests: make(chan *plumbing.ContainerMetricsRequest, 100),
		RecentLogsRequests:       make(chan *plumbing.RecentLogsRequest, 100),
		SubscribeServers:         make(chan plumbing.Doppler_SubscribeServer, 100),
		done:                     make(chan struct{}),
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
	tlsConfig, err := plumbing.NewServerMutualTLSConfig(
		"../fixtures/server.crt",
		"../fixtures/server.key",
		"../fixtures/loggregator-ca.crt",
	)
	if err != nil {
		return err
	}
	transportCreds := credentials.NewTLS(tlsConfig)

	grpcServer := grpc.NewServer(grpc.Creds(transportCreds))
	plumbing.RegisterDopplerServer(grpcServer, fakeDoppler)

	return grpcServer.Serve(fakeDoppler.grpcListener)
}

func (fakeDoppler *FakeDoppler) Stop() {
	fakeDoppler.Lock()
	if fakeDoppler.grpcListener != nil {
		fakeDoppler.grpcListener.Close()
	}
	fakeDoppler.Unlock()
	<-fakeDoppler.done
}

func (fakeDoppler *FakeDoppler) SendLogMessage(messageBody []byte) {
	fakeDoppler.grpcOut <- messageBody
}

func (fakeDoppler *FakeDoppler) CloseLogMessageStream() {
	close(fakeDoppler.grpcOut)
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
