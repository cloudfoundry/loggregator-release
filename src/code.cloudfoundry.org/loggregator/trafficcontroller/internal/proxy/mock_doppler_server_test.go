package proxy_test

import (
	"code.cloudfoundry.org/loggregator/plumbing"

	"golang.org/x/net/context"
	"google.golang.org/grpc/metadata"
)

type mockDopplerServer struct {
	SubscribeCalled chan bool
	SubscribeInput  struct {
		Req    chan *plumbing.SubscriptionRequest
		Stream chan plumbing.Doppler_SubscribeServer
	}
	SubscribeOutput struct {
		Err chan error
	}
	ContainerMetricsCalled chan bool
	ContainerMetricsInput  struct {
		Ctx chan context.Context
		Req chan *plumbing.ContainerMetricsRequest
	}
	ContainerMetricsOutput struct {
		Resp chan *plumbing.ContainerMetricsResponse
		Err  chan error
	}
	RecentLogsCalled chan bool
	RecentLogsInput  struct {
		Ctx chan context.Context
		Req chan *plumbing.RecentLogsRequest
	}
	RecentLogsOutput struct {
		Resp chan *plumbing.RecentLogsResponse
		Err  chan error
	}
}

func newMockDopplerServer() *mockDopplerServer {
	m := &mockDopplerServer{}
	m.SubscribeCalled = make(chan bool, 100)
	m.SubscribeInput.Req = make(chan *plumbing.SubscriptionRequest, 100)
	m.SubscribeInput.Stream = make(chan plumbing.Doppler_SubscribeServer, 100)
	m.SubscribeOutput.Err = make(chan error, 100)
	m.ContainerMetricsCalled = make(chan bool, 100)
	m.ContainerMetricsInput.Ctx = make(chan context.Context, 100)
	m.ContainerMetricsInput.Req = make(chan *plumbing.ContainerMetricsRequest, 100)
	m.ContainerMetricsOutput.Resp = make(chan *plumbing.ContainerMetricsResponse, 100)
	m.ContainerMetricsOutput.Err = make(chan error, 100)
	m.RecentLogsCalled = make(chan bool, 100)
	m.RecentLogsInput.Ctx = make(chan context.Context, 100)
	m.RecentLogsInput.Req = make(chan *plumbing.RecentLogsRequest, 100)
	m.RecentLogsOutput.Resp = make(chan *plumbing.RecentLogsResponse, 100)
	m.RecentLogsOutput.Err = make(chan error, 100)
	return m
}
func (m *mockDopplerServer) Subscribe(req *plumbing.SubscriptionRequest, stream plumbing.Doppler_SubscribeServer) (err error) {
	m.SubscribeCalled <- true
	m.SubscribeInput.Req <- req
	m.SubscribeInput.Stream <- stream
	return <-m.SubscribeOutput.Err
}
func (m *mockDopplerServer) ContainerMetrics(ctx context.Context, req *plumbing.ContainerMetricsRequest) (resp *plumbing.ContainerMetricsResponse, err error) {
	m.ContainerMetricsCalled <- true
	m.ContainerMetricsInput.Ctx <- ctx
	m.ContainerMetricsInput.Req <- req
	return <-m.ContainerMetricsOutput.Resp, <-m.ContainerMetricsOutput.Err
}
func (m *mockDopplerServer) RecentLogs(ctx context.Context, req *plumbing.RecentLogsRequest) (resp *plumbing.RecentLogsResponse, err error) {
	m.RecentLogsCalled <- true
	m.RecentLogsInput.Ctx <- ctx
	m.RecentLogsInput.Req <- req
	return <-m.RecentLogsOutput.Resp, <-m.RecentLogsOutput.Err
}

type mockDoppler_SubscribeServer struct {
	SendCalled chan bool
	SendInput  struct {
		Arg0 chan *plumbing.Response
	}
	SendOutput struct {
		Ret0 chan error
	}
	SendHeaderCalled chan bool
	SendHeaderInput  struct {
		Arg0 chan metadata.MD
	}
	SendHeaderOutput struct {
		Ret0 chan error
	}
	SetTrailerCalled chan bool
	SetTrailerInput  struct {
		Arg0 chan metadata.MD
	}
	ContextCalled chan bool
	ContextOutput struct {
		Ret0 chan context.Context
	}
	SendMsgCalled chan bool
	SendMsgInput  struct {
		M chan interface{}
	}
	SendMsgOutput struct {
		Ret0 chan error
	}
	RecvMsgCalled chan bool
	RecvMsgInput  struct {
		M chan interface{}
	}
	RecvMsgOutput struct {
		Ret0 chan error
	}
}

func newMockDoppler_SubscribeServer() *mockDoppler_SubscribeServer {
	m := &mockDoppler_SubscribeServer{}
	m.SendCalled = make(chan bool, 100)
	m.SendInput.Arg0 = make(chan *plumbing.Response, 100)
	m.SendOutput.Ret0 = make(chan error, 100)
	m.SendHeaderCalled = make(chan bool, 100)
	m.SendHeaderInput.Arg0 = make(chan metadata.MD, 100)
	m.SendHeaderOutput.Ret0 = make(chan error, 100)
	m.SetTrailerCalled = make(chan bool, 100)
	m.SetTrailerInput.Arg0 = make(chan metadata.MD, 100)
	m.ContextCalled = make(chan bool, 100)
	m.ContextOutput.Ret0 = make(chan context.Context, 100)
	m.SendMsgCalled = make(chan bool, 100)
	m.SendMsgInput.M = make(chan interface{}, 100)
	m.SendMsgOutput.Ret0 = make(chan error, 100)
	m.RecvMsgCalled = make(chan bool, 100)
	m.RecvMsgInput.M = make(chan interface{}, 100)
	m.RecvMsgOutput.Ret0 = make(chan error, 100)
	return m
}
func (m *mockDoppler_SubscribeServer) Send(arg0 *plumbing.Response) error {
	m.SendCalled <- true
	m.SendInput.Arg0 <- arg0
	return <-m.SendOutput.Ret0
}
func (m *mockDoppler_SubscribeServer) SendHeader(arg0 metadata.MD) error {
	m.SendHeaderCalled <- true
	m.SendHeaderInput.Arg0 <- arg0
	return <-m.SendHeaderOutput.Ret0
}
func (m *mockDoppler_SubscribeServer) SetTrailer(arg0 metadata.MD) {
	m.SetTrailerCalled <- true
	m.SetTrailerInput.Arg0 <- arg0
}
func (m *mockDoppler_SubscribeServer) Context() context.Context {
	m.ContextCalled <- true
	return <-m.ContextOutput.Ret0
}
func (m *mockDoppler_SubscribeServer) SendMsg(a interface{}) error {
	m.SendMsgCalled <- true
	m.SendMsgInput.M <- a
	return <-m.SendMsgOutput.Ret0
}
func (m *mockDoppler_SubscribeServer) RecvMsg(a interface{}) error {
	m.RecvMsgCalled <- true
	m.RecvMsgInput.M <- a
	return <-m.RecvMsgOutput.Ret0
}

type mockDoppler_FirehoseServer struct {
	SendCalled chan bool
	SendInput  struct {
		Arg0 chan *plumbing.Response
	}
	SendOutput struct {
		Ret0 chan error
	}
	SendHeaderCalled chan bool
	SendHeaderInput  struct {
		Arg0 chan metadata.MD
	}
	SendHeaderOutput struct {
		Ret0 chan error
	}
	SetTrailerCalled chan bool
	SetTrailerInput  struct {
		Arg0 chan metadata.MD
	}
	ContextCalled chan bool
	ContextOutput struct {
		Ret0 chan context.Context
	}
	SendMsgCalled chan bool
	SendMsgInput  struct {
		M chan interface{}
	}
	SendMsgOutput struct {
		Ret0 chan error
	}
	RecvMsgCalled chan bool
	RecvMsgInput  struct {
		M chan interface{}
	}
	RecvMsgOutput struct {
		Ret0 chan error
	}
}

func newMockDoppler_FirehoseServer() *mockDoppler_FirehoseServer {
	m := &mockDoppler_FirehoseServer{}
	m.SendCalled = make(chan bool, 100)
	m.SendInput.Arg0 = make(chan *plumbing.Response, 100)
	m.SendOutput.Ret0 = make(chan error, 100)
	m.SendHeaderCalled = make(chan bool, 100)
	m.SendHeaderInput.Arg0 = make(chan metadata.MD, 100)
	m.SendHeaderOutput.Ret0 = make(chan error, 100)
	m.SetTrailerCalled = make(chan bool, 100)
	m.SetTrailerInput.Arg0 = make(chan metadata.MD, 100)
	m.ContextCalled = make(chan bool, 100)
	m.ContextOutput.Ret0 = make(chan context.Context, 100)
	m.SendMsgCalled = make(chan bool, 100)
	m.SendMsgInput.M = make(chan interface{}, 100)
	m.SendMsgOutput.Ret0 = make(chan error, 100)
	m.RecvMsgCalled = make(chan bool, 100)
	m.RecvMsgInput.M = make(chan interface{}, 100)
	m.RecvMsgOutput.Ret0 = make(chan error, 100)
	return m
}
func (m *mockDoppler_FirehoseServer) Send(arg0 *plumbing.Response) error {
	m.SendCalled <- true
	m.SendInput.Arg0 <- arg0
	return <-m.SendOutput.Ret0
}
func (m *mockDoppler_FirehoseServer) SendHeader(arg0 metadata.MD) error {
	m.SendHeaderCalled <- true
	m.SendHeaderInput.Arg0 <- arg0
	return <-m.SendHeaderOutput.Ret0
}
func (m *mockDoppler_FirehoseServer) SetTrailer(arg0 metadata.MD) {
	m.SetTrailerCalled <- true
	m.SetTrailerInput.Arg0 <- arg0
}
func (m *mockDoppler_FirehoseServer) Context() context.Context {
	m.ContextCalled <- true
	return <-m.ContextOutput.Ret0
}
func (m *mockDoppler_FirehoseServer) SendMsg(a interface{}) error {
	m.SendMsgCalled <- true
	m.SendMsgInput.M <- a
	return <-m.SendMsgOutput.Ret0
}
func (m *mockDoppler_FirehoseServer) RecvMsg(a interface{}) error {
	m.RecvMsgCalled <- true
	m.RecvMsgInput.M <- a
	return <-m.RecvMsgOutput.Ret0
}
