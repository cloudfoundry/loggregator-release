package trafficcontroller_test

import (
	"net"

	"code.cloudfoundry.org/loggregator/plumbing"
	"code.cloudfoundry.org/loggregator/testservers"

	"golang.org/x/net/context"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

type FakeDoppler struct {
	GrpcEndpoint         string
	grpcListener         net.Listener
	grpcOut              chan []byte
	grpcServer           *grpc.Server
	SubscriptionRequests chan *plumbing.SubscriptionRequest
	RecentLogsRequests   chan *plumbing.RecentLogsRequest
	SubscribeServers     chan plumbing.Doppler_BatchSubscribeServer
	done                 chan struct{}
}

func NewFakeDoppler() *FakeDoppler {
	return &FakeDoppler{
		GrpcEndpoint:         "127.0.0.1:0",
		grpcOut:              make(chan []byte, 100),
		SubscriptionRequests: make(chan *plumbing.SubscriptionRequest, 100),
		RecentLogsRequests:   make(chan *plumbing.RecentLogsRequest, 100),
		SubscribeServers:     make(chan plumbing.Doppler_BatchSubscribeServer, 100),
		done:                 make(chan struct{}),
	}
}

func (fakeDoppler *FakeDoppler) Addr() string {
	return fakeDoppler.grpcListener.Addr().String()
}

func (fakeDoppler *FakeDoppler) Start() error {
	var err error
	fakeDoppler.grpcListener, err = net.Listen("tcp", fakeDoppler.GrpcEndpoint)
	if err != nil {
		return err
	}
	tlsConfig, err := plumbing.NewServerMutualTLSConfig(
		testservers.Cert("doppler.crt"),
		testservers.Cert("doppler.key"),
		testservers.Cert("loggregator-ca.crt"),
	)
	if err != nil {
		return err
	}
	transportCreds := credentials.NewTLS(tlsConfig)

	fakeDoppler.grpcServer = grpc.NewServer(grpc.Creds(transportCreds))
	plumbing.RegisterDopplerServer(fakeDoppler.grpcServer, fakeDoppler)

	go func() {
		defer close(fakeDoppler.done)
		fakeDoppler.grpcServer.Serve(fakeDoppler.grpcListener)
	}()
	return nil
}

func (fakeDoppler *FakeDoppler) Stop() {
	if fakeDoppler.grpcServer != nil {
		fakeDoppler.grpcServer.Stop()
	}
	<-fakeDoppler.done
}

func (fakeDoppler *FakeDoppler) SendLogMessage(messageBody []byte) {
	fakeDoppler.grpcOut <- messageBody
}

func (fakeDoppler *FakeDoppler) CloseLogMessageStream() {
	close(fakeDoppler.grpcOut)
}

func (fakeDoppler *FakeDoppler) Subscribe(request *plumbing.SubscriptionRequest, server plumbing.Doppler_SubscribeServer) error {
	panic("not implemented")
}

func (fakeDoppler *FakeDoppler) BatchSubscribe(request *plumbing.SubscriptionRequest, server plumbing.Doppler_BatchSubscribeServer) error {
	fakeDoppler.SubscriptionRequests <- request
	fakeDoppler.SubscribeServers <- server
	for msg := range fakeDoppler.grpcOut {
		err := server.Send(&plumbing.BatchResponse{
			Payload: [][]byte{msg},
		})
		if err != nil {
			return err
		}
	}
	return nil
}

//TODO: Deprecated
func (fakeDoppler *FakeDoppler) ContainerMetrics(ctx context.Context, request *plumbing.ContainerMetricsRequest) (*plumbing.ContainerMetricsResponse, error) {
	return nil, nil
}

func (fakeDoppler *FakeDoppler) RecentLogs(ctx context.Context, request *plumbing.RecentLogsRequest) (*plumbing.RecentLogsResponse, error) {
	fakeDoppler.RecentLogsRequests <- request
	resp := new(plumbing.RecentLogsResponse)
	for msg := range fakeDoppler.grpcOut {
		resp.Payload = append(resp.Payload, msg)
	}

	return resp, nil
}
