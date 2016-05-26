package websocket_test

type fakeAddr struct{}

func (fake fakeAddr) Network() string {
	return "RemoteAddressNetwork"
}

func (fake fakeAddr) String() string {
	return "client-address"
}
