package dopplerwriter_test

import (
	"metron/dopplerwriter"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/cloudfoundry/loggregatorlib/loggertesthelper"
)

var _ = Describe("DopplerWriter", func() {
	It("sends messages to a random doppler", func() {
		clientPool := &mockClientPool{}
		logger := loggertesthelper.Logger()
		s := dopplerwriter.NewDopplerWriter(clientPool, logger)

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

func (m *mockClientPool) RandomClient() (dopplerwriter.Client, error) {
	m.randomClient = &mockClient{}
	return m.randomClient, nil
}

type mockClient struct {
	data [][]byte
}

func (m *mockClient) Send(p []byte) {
	m.data = append(m.data, p)
}
