package v1

import (
	"github.com/cloudfoundry/dropsonde/metricbatcher"
	"github.com/cloudfoundry/dropsonde/metrics"
	"github.com/cloudfoundry/dropsonde/signature"
)

type UDPWrapper struct {
	conn         Conn
	sharedSecret []byte
}

func NewUDPWrapper(conn Conn, sharedSecret []byte) *UDPWrapper {
	return &UDPWrapper{
		conn:         conn,
		sharedSecret: sharedSecret,
	}
}

func (u *UDPWrapper) Write(message []byte, chainers ...metricbatcher.BatchCounterChainer) error {
	signedMessage := signature.SignMessage(message, u.sharedSecret)

	err := u.conn.Write(signedMessage)
	if err != nil {
		// metric-documentation-v1: (udp.sendErrorCount) Total number of errors that have
		// occurred while trying to send envelope to doppler v1 UDP API
		metrics.BatchIncrementCounter("udp.sendErrorCount")
		return err
	}

	// metric-documentation-v1: (udp.sentMessageCount) The number of envelopes sent to
	// dopplers v1 UDP API
	metrics.BatchIncrementCounter("udp.sentMessageCount")

	// metric-documentation-v1: (udp.sentByteCount) The number of bytes sent to
	// dopplers v1 UDP API
	metrics.BatchAddCounter("udp.sentByteCount", uint64(len(message)))

	// metric-documentation-v1: (DopplerForwarder.sentMessages) The number of envelopes sent to
	// dopplers v1 API over all protocols
	metrics.BatchIncrementCounter("DopplerForwarder.sentMessages")
	for _, chainer := range chainers {
		chainer.SetTag("protocol", "udp").Increment()
	}

	return nil
}
