package signer

import (
	"metron/writers"

	"github.com/cloudfoundry/dropsonde/signature"
)

type Signer struct {
	sharedSecret string
	outputWriter writers.ByteArrayWriter
}

func New(sharedSecret string, outputWriter writers.ByteArrayWriter) *Signer {
	return &Signer{
		sharedSecret: sharedSecret,
		outputWriter: outputWriter,
	}
}

func (s *Signer) Write(message []byte) {
	signedMessage := signature.SignMessage(message, []byte(s.sharedSecret))
	s.outputWriter.Write(signedMessage)
}
