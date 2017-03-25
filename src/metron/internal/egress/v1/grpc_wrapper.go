package v1

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
		// metric-documentation-v1: (grpc.sendErrorCount) Total number of errors that have
		// occurred while trying to send envelope to doppler v1 gRPC API
		metrics.BatchIncrementCounter("grpc.sendErrorCount")
		return err
	}

	// metric-documentation-v1: (grpc.sentMessageCount) The number of envelopes sent to
	// dopplers v1 gRPC API
	metrics.BatchIncrementCounter("grpc.sentMessageCount")

	// metric-documentation-v1: (grpc.sentByteCount) The number of bytes sent to
	// dopplers v1 gRPC API
	metrics.BatchAddCounter("grpc.sentByteCount", uint64(len(message)))

	// metric-documentation-v1: (DopplerForwarder.sentMessages) The number of envelopes sent to
	// dopplers v1 API over all protocols
	metrics.BatchIncrementCounter("DopplerForwarder.sentMessages")
	for _, chainer := range chainers {
		chainer.SetTag("protocol", "grpc").Increment()
	}

	return nil
}
