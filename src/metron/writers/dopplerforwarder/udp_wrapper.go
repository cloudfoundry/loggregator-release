package dopplerforwarder

import (
	"github.com/cloudfoundry/dropsonde/metricbatcher"
	"github.com/cloudfoundry/dropsonde/metrics"
	"github.com/cloudfoundry/dropsonde/signature"
	"github.com/cloudfoundry/gosteno"
)

type UDPWrapper struct {
	sharedSecret []byte
	logger       *gosteno.Logger
}

func NewUDPWrapper(sharedSecret []byte, logger *gosteno.Logger) *UDPWrapper {
	return &UDPWrapper{
		sharedSecret: sharedSecret,
		logger:       logger,
	}
}

func (u *UDPWrapper) Write(client Client, message []byte, chainers ...metricbatcher.BatchCounterChainer) error {
	signedMessage := signature.SignMessage(message, u.sharedSecret)

	sentLength, err := client.Write(signedMessage)
	if err != nil {
		u.logger.Errorf("Error writing to UDP client %v\n", err)
		metrics.BatchIncrementCounter("udp.sendErrorCount")
		return err
	}
	metrics.BatchIncrementCounter("udp.sentMessageCount")
	metrics.BatchAddCounter("udp.sentByteCount", uint64(sentLength))

	// The TLS side writes this metric in the batch.Writer.  For UDP,
	// it needs to be done here.
	metrics.BatchIncrementCounter("DopplerForwarder.sentMessages")
	for _, chainer := range chainers {
		chainer.SetTag("protocol", "udp").Increment()
	}

	return nil
}
