package grpcconnector_test

import (
	"plumbing"

	"golang.org/x/net/context"
)

type mockDopplerServer struct {
	streamInputRequests chan *plumbing.StreamRequest
	streamInputServers  chan plumbing.Doppler_StreamServer
	streamOutput        chan error

	firehoseInputRequests chan *plumbing.FirehoseRequest
	firehoseInputServers  chan plumbing.Doppler_FirehoseServer
	firehoseOutput        chan error

	containerMetricsRequests    chan *plumbing.ContainerMetricsRequest
	containerMetricsOutputResps chan *plumbing.ContainerMetricsResponse
	containerMetricsOutputErrs  chan error
}

func newMockDopplerServer() *mockDopplerServer {
	return &mockDopplerServer{
		streamInputRequests: make(chan *plumbing.StreamRequest, 100),
		streamInputServers:  make(chan plumbing.Doppler_StreamServer, 100),
		streamOutput:        make(chan error, 100),

		firehoseInputRequests: make(chan *plumbing.FirehoseRequest, 100),
		firehoseInputServers:  make(chan plumbing.Doppler_FirehoseServer, 100),
		firehoseOutput:        make(chan error, 100),

		containerMetricsRequests:    make(chan *plumbing.ContainerMetricsRequest, 100),
		containerMetricsOutputResps: make(chan *plumbing.ContainerMetricsResponse, 100),
		containerMetricsOutputErrs:  make(chan error, 100),
	}
}

func (m *mockDopplerServer) Stream(req *plumbing.StreamRequest, server plumbing.Doppler_StreamServer) error {
	m.streamInputRequests <- req
	m.streamInputServers <- server
	return <-m.streamOutput
}

func (m *mockDopplerServer) Firehose(req *plumbing.FirehoseRequest, server plumbing.Doppler_FirehoseServer) error {
	m.firehoseInputRequests <- req
	m.firehoseInputServers <- server
	return <-m.firehoseOutput
}

func (m *mockDopplerServer) ContainerMetrics(ctx context.Context, req *plumbing.ContainerMetricsRequest) (*plumbing.ContainerMetricsResponse, error) {
	m.containerMetricsRequests <- req
	return <-m.containerMetricsOutputResps, <-m.containerMetricsOutputErrs
}
