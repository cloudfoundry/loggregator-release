package channel_group_connector_test

type mockFinder struct {
	WebsocketServersCalled chan bool
	WebsocketServersOutput struct {
		ret0 chan []string
	}
}

func newMockFinder() *mockFinder {
	m := &mockFinder{}
	m.WebsocketServersCalled = make(chan bool, 100)
	m.WebsocketServersOutput.ret0 = make(chan []string, 100)
	return m
}
func (m *mockFinder) WebsocketServers() []string {
	m.WebsocketServersCalled <- true
	return <-m.WebsocketServersOutput.ret0
}
