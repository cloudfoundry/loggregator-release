package dopplerforwarder

import (
	"github.com/cloudfoundry/dropsonde/metricbatcher"
	"github.com/cloudfoundry/dropsonde/metrics"
	"github.com/cloudfoundry/dropsonde/signature"
)

type Conn interface {
	Write(data []byte) error
}

type GRPCWrapper struct {
	conn         Conn
	sharedSecret []byte
}

func NewGRPCWrapper(conn Conn, sharedSecret []byte) *GRPCWrapper {
	return &GRPCWrapper{
		conn:         conn,
		sharedSecret: sharedSecret,
	}
}

func (u *GRPCWrapper) Write(message []byte, chainers ...metricbatcher.BatchCounterChainer) error {
	signedMessage := signature.SignMessage(message, u.sharedSecret)

	err := u.conn.Write(signedMessage)
	if err != nil {
		metrics.BatchIncrementCounter("grpc.sendErrorCount")
		return err
	}
	metrics.BatchIncrementCounter("grpc.sentMessageCount")
	metrics.BatchAddCounter("grpc.sentByteCount", uint64(len(message)))

	metrics.BatchIncrementCounter("DopplerForwarder.sentMessages")
	for _, chainer := range chainers {
		chainer.SetTag("protocol", "grpc").Increment()
	}

	return nil
}
