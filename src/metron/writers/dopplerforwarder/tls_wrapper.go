package dopplerforwarder

import (
	"github.com/cloudfoundry/dropsonde/metrics"
	"github.com/cloudfoundry/gosteno"

	"encoding/binary"
)

type TLSWrapper struct {
	logger *gosteno.Logger
}

func NewTLSWrapper(logger *gosteno.Logger) *TLSWrapper {
	return &TLSWrapper{
		logger: logger,
	}
}

func (t *TLSWrapper) Write(client Client, message []byte) error {
	err := binary.Write(client, binary.LittleEndian, uint32(len(message)))
	if err != nil {
		t.logger.Errorf("Error encoding message length: %v\n", err)
		metrics.BatchIncrementCounter("tls.sendErrorCount")
		client.Close()
		return err
	}

	sentBytes, err := client.Write(message)
	if err != nil {
		t.logger.Errorf("Error writing to TLS client %v\n", err)
		metrics.BatchIncrementCounter("tls.sendErrorCount")
		client.Close()
		return err
	}
	metrics.BatchAddCounter("tls.sentByteCount", uint64(sentBytes)+4)
	metrics.BatchIncrementCounter("tls.sentMessageCount")

	return nil
}
