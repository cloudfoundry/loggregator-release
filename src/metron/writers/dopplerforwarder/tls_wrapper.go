package dopplerforwarder

import "github.com/cloudfoundry/dropsonde/metrics"

type TLSWrapper struct{}

func NewTLSWrapper() *TLSWrapper {
	return &TLSWrapper{}
}

func (t *TLSWrapper) Write(client Client, message []byte) error {
	sentBytes, err := client.Write(message)
	if err != nil {
		metrics.BatchIncrementCounter("tls.sendErrorCount")
		client.Close()
		return err
	}
	metrics.BatchAddCounter("tls.sentByteCount", uint64(sentBytes))
	metrics.BatchIncrementCounter("tls.sentMessageCount")
	return nil
}
