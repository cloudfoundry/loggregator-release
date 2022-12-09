package app_test

import (
	"code.cloudfoundry.org/loggregator-release/plumbing"
)

type mockDopplerServer struct {
	plumbing.DopplerServer

	SubscribeCalled chan bool
	SubscribeInput  struct {
		Req    chan *plumbing.SubscriptionRequest
		Stream chan plumbing.Doppler_SubscribeServer
	}
	SubscribeOutput struct {
		Err chan error
	}
	BatchSubscribeCalled chan bool
	BatchSubscribeInput  struct {
		Req    chan *plumbing.SubscriptionRequest
		Stream chan plumbing.Doppler_BatchSubscribeServer
	}
	BatchSubscribeOutput struct {
		Err chan error
	}
}

func newMockDopplerServer() *mockDopplerServer {
	m := &mockDopplerServer{}
	m.SubscribeCalled = make(chan bool, 100)
	m.SubscribeInput.Req = make(chan *plumbing.SubscriptionRequest, 100)
	m.SubscribeInput.Stream = make(chan plumbing.Doppler_SubscribeServer, 100)
	m.SubscribeOutput.Err = make(chan error, 100)
	m.BatchSubscribeCalled = make(chan bool, 100)
	m.BatchSubscribeInput.Req = make(chan *plumbing.SubscriptionRequest, 100)
	m.BatchSubscribeInput.Stream = make(chan plumbing.Doppler_BatchSubscribeServer, 100)
	m.BatchSubscribeOutput.Err = make(chan error, 100)
	return m
}
func (m *mockDopplerServer) Subscribe(req *plumbing.SubscriptionRequest, stream plumbing.Doppler_SubscribeServer) (err error) {
	m.SubscribeCalled <- true
	m.SubscribeInput.Req <- req
	m.SubscribeInput.Stream <- stream
	return <-m.SubscribeOutput.Err
}

func (m *mockDopplerServer) BatchSubscribe(req *plumbing.SubscriptionRequest, stream plumbing.Doppler_BatchSubscribeServer) (err error) {
	m.BatchSubscribeCalled <- true
	m.BatchSubscribeInput.Req <- req
	m.BatchSubscribeInput.Stream <- stream
	return <-m.BatchSubscribeOutput.Err
}
