package dopplerforwarder

import (
	"github.com/cloudfoundry/dropsonde/metrics"
	"github.com/cloudfoundry/dropsonde/signature"
)

type UDPWrapper struct {
	sharedSecret []byte
}

func NewUDPWrapper(sharedSecret []byte) *UDPWrapper {
	return &UDPWrapper{sharedSecret: sharedSecret}
}

func (u *UDPWrapper) Write(client Client, message []byte) error {
	signedMessage := signature.SignMessage(message, u.sharedSecret)

	sentLength, err := client.Write(signedMessage)
	if err != nil {
		metrics.BatchIncrementCounter("udp.sendErrorCount")
		return err
	}
	metrics.BatchIncrementCounter("udp.sentMessageCount")
	metrics.BatchAddCounter("udp.sentByteCount", uint64(sentLength))
	return nil
}
