package batch_test

type mockCongestionError struct {
	message, doppler string
}

func (m mockCongestionError) Error() string {
	return m.message
}

func (m mockCongestionError) CongestedDoppler() string {
	return m.doppler
}
