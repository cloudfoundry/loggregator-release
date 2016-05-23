package dopplerforwarder

import (
	"github.com/cloudfoundry/dropsonde/metricbatcher"
	"github.com/cloudfoundry/dropsonde/metrics"
	"github.com/cloudfoundry/gosteno"
)

type Wrapper struct {
	logger   *gosteno.Logger
	protocol string
}

func NewWrapper(logger *gosteno.Logger, protocol string) *Wrapper {
	switch protocol {
	case "tcp", "tls":
	default:
		panic("Invalid protocol specified when creating NewWrapper")
	}
	return &Wrapper{
		logger:   logger,
		protocol: protocol,
	}
}

func (w *Wrapper) Write(client Client, message []byte, chainers ...metricbatcher.BatchCounterChainer) error {
	sentBytes, err := client.Write(message)

	if err != nil {
		w.logger.Errorf("Error writing to %s client %v\n", w.protocol, err)
		metrics.BatchIncrementCounter(w.protocol + ".sendErrorCount")
		client.Close()
		return err
	}

	metrics.BatchAddCounter(w.protocol+".sentByteCount", uint64(sentBytes))
	for _, chainer := range chainers {
		chainer.SetTag("protocol", w.protocol).Increment()
	}

	return nil
}
