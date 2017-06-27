package v1_test

import (
	"io"

	"code.cloudfoundry.org/loggregator/dopplerservice"
	"code.cloudfoundry.org/loggregator/plumbing"

	"golang.org/x/net/context"

	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

type mockEventer struct {
	NextCalled chan bool
	NextOutput struct {
		Ret0 chan dopplerservice.Event
	}
}

func newMockEventer() *mockEventer {
	m := &mockEventer{}
	m.NextCalled = make(chan bool, 100)
	m.NextOutput.Ret0 = make(chan dopplerservice.Event, 100)
	return m
}
func (m *mockEventer) Next() dopplerservice.Event {
	m.NextCalled <- true
	return <-m.NextOutput.Ret0
}

type mockConn struct {
	WriteCalled chan bool
	WriteInput  struct {
		Data chan []byte
	}
	WriteOutput struct {
		Err chan error
	}
}

func newMockConn() *mockConn {
	m := &mockConn{}
	m.WriteCalled = make(chan bool, 100)
	m.WriteInput.Data = make(chan []byte, 100)
	m.WriteOutput.Err = make(chan error, 100)
	return m
}
func (m *mockConn) Write(data []byte) (err error) {
	m.WriteCalled <- true
	m.WriteInput.Data <- data
	return <-m.WriteOutput.Err
}

type mockCloser struct {
	CloseCalled chan bool
	CloseOutput struct {
		Ret0 chan error
	}
}

func newMockCloser() *mockCloser {
	m := &mockCloser{}
	m.CloseCalled = make(chan bool, 100)
	m.CloseOutput.Ret0 = make(chan error, 100)
	return m
}
func (m *mockCloser) Close() error {
	m.CloseCalled <- true
	return <-m.CloseOutput.Ret0
}

type mockPusher struct {
	PusherCalled chan bool
	PusherInput  struct {
		Ctx  chan context.Context
		Opts chan []grpc.CallOption
	}
	PusherOutput struct {
		Ret0 chan plumbing.DopplerIngestor_PusherClient
		Ret1 chan error
	}
}

func newMockPusher() *mockPusher {
	m := &mockPusher{}
	m.PusherCalled = make(chan bool, 100)
	m.PusherInput.Ctx = make(chan context.Context, 100)
	m.PusherInput.Opts = make(chan []grpc.CallOption, 100)
	m.PusherOutput.Ret0 = make(chan plumbing.DopplerIngestor_PusherClient, 100)
	m.PusherOutput.Ret1 = make(chan error, 100)
	return m
}
func (m *mockPusher) Pusher(ctx context.Context, opts ...grpc.CallOption) (plumbing.DopplerIngestor_PusherClient, error) {
	m.PusherCalled <- true
	m.PusherInput.Ctx <- ctx
	m.PusherInput.Opts <- opts
	return <-m.PusherOutput.Ret0, <-m.PusherOutput.Ret1
}

type mockDopplerIngestor_PusherClient struct {
	SendCalled chan bool
	SendInput  struct {
		Arg0 chan *plumbing.EnvelopeData
	}
	SendOutput struct {
		Ret0 chan error
	}
	CloseAndRecvCalled chan bool
	CloseAndRecvOutput struct {
		Ret0 chan *plumbing.PushResponse
		Ret1 chan error
	}
	HeaderCalled chan bool
	HeaderOutput struct {
		Ret0 chan metadata.MD
		Ret1 chan error
	}
	TrailerCalled chan bool
	TrailerOutput struct {
		Ret0 chan metadata.MD
	}
	CloseSendCalled chan bool
	CloseSendOutput struct {
		Ret0 chan error
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

func newMockDopplerIngestor_PusherClient() *mockDopplerIngestor_PusherClient {
	m := &mockDopplerIngestor_PusherClient{}
	m.SendCalled = make(chan bool, 100)
	m.SendInput.Arg0 = make(chan *plumbing.EnvelopeData, 100)
	m.SendOutput.Ret0 = make(chan error, 100)
	m.CloseAndRecvCalled = make(chan bool, 100)
	m.CloseAndRecvOutput.Ret0 = make(chan *plumbing.PushResponse, 100)
	m.CloseAndRecvOutput.Ret1 = make(chan error, 100)
	m.HeaderCalled = make(chan bool, 100)
	m.HeaderOutput.Ret0 = make(chan metadata.MD, 100)
	m.HeaderOutput.Ret1 = make(chan error, 100)
	m.TrailerCalled = make(chan bool, 100)
	m.TrailerOutput.Ret0 = make(chan metadata.MD, 100)
	m.CloseSendCalled = make(chan bool, 100)
	m.CloseSendOutput.Ret0 = make(chan error, 100)
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
func (m *mockDopplerIngestor_PusherClient) Send(arg0 *plumbing.EnvelopeData) error {
	m.SendCalled <- true
	m.SendInput.Arg0 <- arg0
	return <-m.SendOutput.Ret0
}
func (m *mockDopplerIngestor_PusherClient) CloseAndRecv() (*plumbing.PushResponse, error) {
	m.CloseAndRecvCalled <- true
	return <-m.CloseAndRecvOutput.Ret0, <-m.CloseAndRecvOutput.Ret1
}
func (m *mockDopplerIngestor_PusherClient) Header() (metadata.MD, error) {
	m.HeaderCalled <- true
	return <-m.HeaderOutput.Ret0, <-m.HeaderOutput.Ret1
}
func (m *mockDopplerIngestor_PusherClient) Trailer() metadata.MD {
	m.TrailerCalled <- true
	return <-m.TrailerOutput.Ret0
}
func (m *mockDopplerIngestor_PusherClient) CloseSend() error {
	m.CloseSendCalled <- true
	return <-m.CloseSendOutput.Ret0
}
func (m *mockDopplerIngestor_PusherClient) Context() context.Context {
	m.ContextCalled <- true
	return <-m.ContextOutput.Ret0
}
func (m *mockDopplerIngestor_PusherClient) SendMsg(m_ interface{}) error {
	m.SendMsgCalled <- true
	m.SendMsgInput.M <- m_
	return <-m.SendMsgOutput.Ret0
}
func (m *mockDopplerIngestor_PusherClient) RecvMsg(m_ interface{}) error {
	m.RecvMsgCalled <- true
	m.RecvMsgInput.M <- m_
	return <-m.RecvMsgOutput.Ret0
}

type mockConnector struct {
	ConnectCalled chan bool
	ConnectOutput struct {
		Ret0 chan io.Closer
		Ret1 chan plumbing.DopplerIngestor_PusherClient
		Ret2 chan error
	}
}

func newMockConnector() *mockConnector {
	m := &mockConnector{}
	m.ConnectCalled = make(chan bool, 100)
	m.ConnectOutput.Ret0 = make(chan io.Closer, 100)
	m.ConnectOutput.Ret1 = make(chan plumbing.DopplerIngestor_PusherClient, 100)
	m.ConnectOutput.Ret2 = make(chan error, 100)
	return m
}
func (m *mockConnector) Connect() (io.Closer, plumbing.DopplerIngestor_PusherClient, error) {
	m.ConnectCalled <- true
	return <-m.ConnectOutput.Ret0, <-m.ConnectOutput.Ret1, <-m.ConnectOutput.Ret2
}
