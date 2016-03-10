package clientreader_test

type mockClientPool struct {
	SetAddressesCalled chan bool
	SetAddressesInput  struct {
		addresses chan []string
	}
	SetAddressesOutput struct {
		ret0 chan int
	}
}

func newMockClientPool() *mockClientPool {
	m := &mockClientPool{}
	m.SetAddressesCalled = make(chan bool, 100)
	m.SetAddressesInput.addresses = make(chan []string, 100)
	m.SetAddressesOutput.ret0 = make(chan int, 100)
	return m
}
func (m *mockClientPool) SetAddresses(addresses []string) int {
	m.SetAddressesCalled <- true
	m.SetAddressesInput.addresses <- addresses
	return <-m.SetAddressesOutput.ret0
}
