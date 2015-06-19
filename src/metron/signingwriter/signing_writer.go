package signingwriter

import (
	"io"
	"github.com/cloudfoundry/dropsonde/signature"
)

type SigningWriter struct {
	sharedSecret   string
	outputWriter   io.Writer
}

func NewSigningWriter(sharedSecret string, outputWriter io.Writer) *SigningWriter {
	return &SigningWriter{
		sharedSecret:   sharedSecret,
		outputWriter:   outputWriter,
	}
}

func (s *SigningWriter) Write(message []byte) {
	signedMessage := signature.SignMessage(message, []byte(s.sharedSecret))
	s.outputWriter.Write(signedMessage)
}
