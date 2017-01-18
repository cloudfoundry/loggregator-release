package clientpool_test

import (
	"plumbing/v2"

	"golang.org/x/net/context"

	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

type mockDopplerIngressClient struct {
	SenderCalled chan bool
	SenderInput  struct {
		Ctx  chan context.Context
		Opts chan []grpc.CallOption
	}
	SenderOutput struct {
		Ret0 chan loggregator.DopplerIngress_SenderClient
		Ret1 chan error
	}
}

func newMockDopplerIngressClient() *mockDopplerIngressClient {
	m := &mockDopplerIngressClient{}
	m.SenderCalled = make(chan bool, 100)
	m.SenderInput.Ctx = make(chan context.Context, 100)
	m.SenderInput.Opts = make(chan []grpc.CallOption, 100)
	m.SenderOutput.Ret0 = make(chan loggregator.DopplerIngress_SenderClient, 100)
	m.SenderOutput.Ret1 = make(chan error, 100)
	return m
}
func (m *mockDopplerIngressClient) Sender(ctx context.Context, opts ...grpc.CallOption) (loggregator.DopplerIngress_SenderClient, error) {
	m.SenderCalled <- true
	m.SenderInput.Ctx <- ctx
	m.SenderInput.Opts <- opts
	return <-m.SenderOutput.Ret0, <-m.SenderOutput.Ret1
}

type mockDopplerIngress_SenderClient struct {
	SendCalled chan bool
	SendInput  struct {
		Arg0 chan *loggregator.Envelope
	}
	SendOutput struct {
		Ret0 chan error
	}
	CloseAndRecvCalled chan bool
	CloseAndRecvOutput struct {
		Ret0 chan *loggregator.SenderResponse
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
	m.SendInput.Arg0 = make(chan *loggregator.Envelope, 100)
	m.SendOutput.Ret0 = make(chan error, 100)
	m.CloseAndRecvCalled = make(chan bool, 100)
	m.CloseAndRecvOutput.Ret0 = make(chan *loggregator.SenderResponse, 100)
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
func (m *mockDopplerIngress_SenderClient) Send(arg0 *loggregator.Envelope) error {
	m.SendCalled <- true
	m.SendInput.Arg0 <- arg0
	return <-m.SendOutput.Ret0
}
func (m *mockDopplerIngress_SenderClient) CloseAndRecv() (*loggregator.SenderResponse, error) {
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
