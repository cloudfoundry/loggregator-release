// This file was generated by github.com/nelsam/hel.  Do not
// edit this code by hand unless you *really* know what you're
// doing.  Expect any changes made manually to be overwritten
// the next time hel regenerates this file.

package proxy_test

import (
	"code.cloudfoundry.org/loggregator-release/plumbing"
	"golang.org/x/net/context"
)

type mockReceiver struct {
	RecvCalled chan bool
	RecvOutput struct {
		Ret0 chan []byte
		Ret1 chan error
	}
}

func newMockReceiver() *mockReceiver {
	m := &mockReceiver{}
	m.RecvCalled = make(chan bool, 100)
	m.RecvOutput.Ret0 = make(chan []byte, 100)
	m.RecvOutput.Ret1 = make(chan error, 100)
	return m
}
func (m *mockReceiver) Recv() ([]byte, error) {
	m.RecvCalled <- true
	return <-m.RecvOutput.Ret0, <-m.RecvOutput.Ret1
}

type mockGrpcConnector struct {
	SubscribeCalled chan bool
	SubscribeInput  struct {
		Ctx chan context.Context
		Req chan *plumbing.SubscriptionRequest
	}
	SubscribeOutput struct {
		Ret0 chan func() ([]byte, error)
		Ret1 chan error
	}
}

func newMockGrpcConnector() *mockGrpcConnector {
	m := &mockGrpcConnector{}
	m.SubscribeCalled = make(chan bool, 100)
	m.SubscribeInput.Ctx = make(chan context.Context, 100)
	m.SubscribeInput.Req = make(chan *plumbing.SubscriptionRequest, 100)
	m.SubscribeOutput.Ret0 = make(chan func() ([]byte, error), 100)
	m.SubscribeOutput.Ret1 = make(chan error, 100)
	return m
}
func (m *mockGrpcConnector) Subscribe(ctx context.Context, req *plumbing.SubscriptionRequest) (func() ([]byte, error), error) {
	m.SubscribeCalled <- true
	m.SubscribeInput.Ctx <- ctx
	m.SubscribeInput.Req <- req
	return <-m.SubscribeOutput.Ret0, <-m.SubscribeOutput.Ret1
}
