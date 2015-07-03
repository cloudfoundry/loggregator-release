package signer_test

import (
	"metron/writers/signer"

	"github.com/cloudfoundry/dropsonde/signature"

	"metron/writers/mocks"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Signer", func() {
	It("sends signed messages to output writer", func() {
		writer := &mocks.MockByteArrayWriter{}
		s := signer.New("shared-secret", writer)

		message := []byte("Some message")
		s.Write(message)

		Expect(writer.Data()).To(HaveLen(1))

		signedMessage := signature.SignMessage(message, []byte("shared-secret"))
		Expect(writer.Data()[0]).To(Equal(signedMessage))
	})
})
