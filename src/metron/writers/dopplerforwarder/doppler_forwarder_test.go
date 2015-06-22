package dopplerforwarder_test

import (
	"metron/writers/dopplerforwarder"

	"github.com/cloudfoundry/loggregatorlib/loggertesthelper"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("DopplerForwarder", func() {
	It("sends messages to a random doppler", func() {
		clientPool := &mockClientPool{}
		logger := loggertesthelper.Logger()
		s := dopplerforwarder.New(clientPool, logger)

		message := []byte("Some message")
		s.Write(message)

		Expect(clientPool.randomClient).ToNot(BeNil())

		data := clientPool.randomClient.data
		Expect(data).To(HaveLen(1))
		Expect(data[0]).To(Equal(message))
	})
})

type mockClientPool struct {
	randomClient *mockClient
}

func (m *mockClientPool) RandomClient() (dopplerforwarder.Client, error) {
	m.randomClient = &mockClient{}
	return m.randomClient, nil
}

type mockClient struct {
	data [][]byte
}

func (m *mockClient) Send(p []byte) {
	m.data = append(m.data, p)
}
