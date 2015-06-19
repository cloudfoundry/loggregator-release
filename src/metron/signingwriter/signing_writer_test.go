package signingwriter_test

import (
	"metron/signingwriter"
	"github.com/cloudfoundry/dropsonde/signature"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("SigningWriter", func() {
	It("sends signed messages to output writer", func() {
		writer := &mockWriter{}
		s := signingwriter.NewSigningWriter("shared-secret", writer)

		message := []byte("Some message")
		s.Write(message)

		Expect(writer.data).To(HaveLen(1))

		signedMessage := signature.SignMessage(message, []byte("shared-secret"))
		Expect(writer.data[0]).To(Equal(signedMessage))
	})
})

type mockWriter struct {
	data [][]byte
}

func (m *mockWriter) Write(p []byte) (bytesWritten int, err error) {
	m.data = append(m.data, p)
	return len(p), nil
}
