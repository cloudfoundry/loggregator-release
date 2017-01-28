package egress

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
		metrics.BatchIncrementCounter("udp.sendErrorCount")
		return err
	}
	metrics.BatchIncrementCounter("udp.sentMessageCount")
	metrics.BatchAddCounter("udp.sentByteCount", uint64(len(message)))

	// The TLS side writes this metric in the batch.Writer.  For UDP,
	// it needs to be done here.
	metrics.BatchIncrementCounter("DopplerForwarder.sentMessages")
	for _, chainer := range chainers {
		chainer.SetTag("protocol", "udp").Increment()
	}

	return nil
}
