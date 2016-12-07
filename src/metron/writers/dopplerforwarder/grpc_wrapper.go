package dopplerforwarder

import (
	"github.com/cloudfoundry/dropsonde/metricbatcher"
	"github.com/cloudfoundry/dropsonde/metrics"
)

type Conn interface {
	Write(data []byte) error
}

type GRPCWrapper struct {
	conn Conn
}

func NewGRPCWrapper(conn Conn) *GRPCWrapper {
	return &GRPCWrapper{
		conn: conn,
	}
}

func (u *GRPCWrapper) Write(message []byte, chainers ...metricbatcher.BatchCounterChainer) error {
	err := u.conn.Write(message)
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
