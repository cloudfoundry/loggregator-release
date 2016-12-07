package listeners_test

type mockAlerter struct {
	AlertCalled chan bool
	AlertInput  struct {
		Missed chan int
	}
}

func newMockAlerter() *mockAlerter {
	m := &mockAlerter{}
	m.AlertCalled = make(chan bool, 100)
	m.AlertInput.Missed = make(chan int, 100)
	return m
}

func (m *mockAlerter) Alert(missed int) {
	m.AlertCalled <- true
	m.AlertInput.Missed <- missed
}
