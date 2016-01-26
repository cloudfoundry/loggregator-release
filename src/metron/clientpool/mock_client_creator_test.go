package clientpool_test

import "metron/clientpool"

type mockClientCreator struct {
	CreateClientCalled chan bool
	CreateClientInput  struct {
		url chan string
	}
	CreateClientOutput struct {
		client chan clientpool.Client
		err    chan error
	}
}

func newMockClientCreator() *mockClientCreator {
	m := &mockClientCreator{}
	m.CreateClientCalled = make(chan bool, 100)
	m.CreateClientInput.url = make(chan string, 100)
	m.CreateClientOutput.client = make(chan clientpool.Client, 100)
	m.CreateClientOutput.err = make(chan error, 100)
	return m
}
func (m *mockClientCreator) CreateClient(url string) (client clientpool.Client, err error) {
	m.CreateClientCalled <- true
	m.CreateClientInput.url <- url
	return <-m.CreateClientOutput.client, <-m.CreateClientOutput.err
}
