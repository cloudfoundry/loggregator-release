package clientpool_test

import (
	"io"
	clientpool "metron/clientpool/v2"
	v2 "plumbing/v2"

	"golang.org/x/net/context"

	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

type mockV2Connector struct {
	ConnectCalled chan bool
	ConnectOutput struct {
		Ret0 chan io.Closer
		Ret1 chan v2.DopplerIngress_SenderClient
		Ret2 chan error
	}
}

func newMockV2Connector() *mockV2Connector {
	m := &mockV2Connector{}
	m.ConnectCalled = make(chan bool, 100)
	m.ConnectOutput.Ret0 = make(chan io.Closer, 100)
	m.ConnectOutput.Ret1 = make(chan v2.DopplerIngress_SenderClient, 100)
	m.ConnectOutput.Ret2 = make(chan error, 100)
	return m
}
func (m *mockV2Connector) Connect() (io.Closer, v2.DopplerIngress_SenderClient, error) {
	m.ConnectCalled <- true
	return <-m.ConnectOutput.Ret0, <-m.ConnectOutput.Ret1, <-m.ConnectOutput.Ret2
}

type mockDopplerIngressClient struct {
	SenderCalled chan bool
	SenderInput  struct {
		Ctx  chan context.Context
		Opts chan []grpc.CallOption
	}
	SenderOutput struct {
		Ret0 chan v2.DopplerIngress_SenderClient
		Ret1 chan error
	}
}

func newMockDopplerIngressClient() *mockDopplerIngressClient {
	m := &mockDopplerIngressClient{}
	m.SenderCalled = make(chan bool, 100)
	m.SenderInput.Ctx = make(chan context.Context, 100)
	m.SenderInput.Opts = make(chan []grpc.CallOption, 100)
	m.SenderOutput.Ret0 = make(chan v2.DopplerIngress_SenderClient, 100)
	m.SenderOutput.Ret1 = make(chan error, 100)
	return m
}
func (m *mockDopplerIngressClient) Sender(ctx context.Context, opts ...grpc.CallOption) (v2.DopplerIngress_SenderClient, error) {
	m.SenderCalled <- true
	m.SenderInput.Ctx <- ctx
	m.SenderInput.Opts <- opts
	return <-m.SenderOutput.Ret0, <-m.SenderOutput.Ret1
}

type mockDopplerIngress_SenderClient struct {
	SendCalled chan bool
	SendInput  struct {
		Arg0 chan *v2.Envelope
	}
	SendOutput struct {
		Ret0 chan error
	}
	CloseAndRecvCalled chan bool
	CloseAndRecvOutput struct {
		Ret0 chan *v2.SenderResponse
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

func newMockDopplerIngress_SenderClient() *mockDopplerIngress_SenderClient {
	m := &mockDopplerIngress_SenderClient{}
	m.SendCalled = make(chan bool, 100)
	m.SendInput.Arg0 = make(chan *v2.Envelope, 100)
	m.SendOutput.Ret0 = make(chan error, 100)
	m.CloseAndRecvCalled = make(chan bool, 100)
	m.CloseAndRecvOutput.Ret0 = make(chan *v2.SenderResponse, 100)
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
func (m *mockDopplerIngress_SenderClient) Send(arg0 *v2.Envelope) error {
	m.SendCalled <- true
	m.SendInput.Arg0 <- arg0
	return <-m.SendOutput.Ret0
}
func (m *mockDopplerIngress_SenderClient) CloseAndRecv() (*v2.SenderResponse, error) {
	m.CloseAndRecvCalled <- true
	return <-m.CloseAndRecvOutput.Ret0, <-m.CloseAndRecvOutput.Ret1
}
func (m *mockDopplerIngress_SenderClient) Header() (metadata.MD, error) {
	m.HeaderCalled <- true
	return <-m.HeaderOutput.Ret0, <-m.HeaderOutput.Ret1
}
func (m *mockDopplerIngress_SenderClient) Trailer() metadata.MD {
	m.TrailerCalled <- true
	return <-m.TrailerOutput.Ret0
}
func (m *mockDopplerIngress_SenderClient) CloseSend() error {
	m.CloseSendCalled <- true
	return <-m.CloseSendOutput.Ret0
}
func (m *mockDopplerIngress_SenderClient) Context() context.Context {
	m.ContextCalled <- true
	return <-m.ContextOutput.Ret0
}
func (m *mockDopplerIngress_SenderClient) SendMsg(m_ interface{}) error {
	m.SendMsgCalled <- true
	m.SendMsgInput.M <- m_
	return <-m.SendMsgOutput.Ret0
}
func (m *mockDopplerIngress_SenderClient) RecvMsg(m_ interface{}) error {
	m.RecvMsgCalled <- true
	m.RecvMsgInput.M <- m_
	return <-m.RecvMsgOutput.Ret0
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

type mockDialFunc struct {
	inputDoppler     chan string
	inputDialOptions chan []grpc.DialOption
	retClientConn    chan *grpc.ClientConn
	retErr           chan error
	fn               clientpool.DialFunc
}

func newMockDialFunc() *mockDialFunc {
	df := &mockDialFunc{
		inputDoppler:     make(chan string, 100),
		inputDialOptions: make(chan []grpc.DialOption, 100),
		retClientConn:    make(chan *grpc.ClientConn, 100),
		retErr:           make(chan error, 100),
	}
	df.fn = func(doppler string, opts ...grpc.DialOption) (*grpc.ClientConn, error) {
		df.inputDoppler <- doppler
		df.inputDialOptions <- opts
		return <-df.retClientConn, <-df.retErr
	}
	return df
}

type mockConn struct {
	WriteCalled chan bool
	WriteInput  struct {
		Data chan *v2.Envelope
	}
	WriteOutput struct {
		Err chan error
	}
}

func newMockConn() *mockConn {
	m := &mockConn{}
	m.WriteCalled = make(chan bool, 100)
	m.WriteInput.Data = make(chan *v2.Envelope, 100)
	m.WriteOutput.Err = make(chan error, 100)
	return m
}
func (m *mockConn) Write(data *v2.Envelope) (err error) {
	m.WriteCalled <- true
	m.WriteInput.Data <- data
	return <-m.WriteOutput.Err
}
