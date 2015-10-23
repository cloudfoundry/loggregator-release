package messagewriter

import (
	"fmt"
	"net"

	"github.com/cloudfoundry/dropsonde/signature"

	"metron/writers"
	"tools/benchmark/metricsreporter"
)

type messageWriter struct {
	writer writers.ByteArrayWriter
}

type networkWriter struct {
	conn        net.Conn
	sentCounter *metricsreporter.Counter
}

type MessageWriterFunc func(message []byte)

func (f MessageWriterFunc) Write(message []byte) {
	f(message)
}

func (nw networkWriter) Write(message []byte) {
	n, err := nw.conn.Write(message)
	if err != nil {
		fmt.Printf("SEND Error: %s\n", err.Error())
		return
	}
	if n < len(message) {
		fmt.Printf("SEND Warning: Tried to send %d bytes but only sent %d\n", len(message), n)
		return
	}
	nw.sentCounter.IncrementValue()
}

func NewMessageWriter(host string, port int, sharedSecret string, sentCounter *metricsreporter.Counter) *messageWriter {
	output, err := net.Dial("udp4", fmt.Sprintf("%s:%d", host, port))
	if err != nil {
		fmt.Printf("DIAL Error: %s\n", err.Error())
	}

	var writer writers.ByteArrayWriter
	writer = networkWriter{
		sentCounter: sentCounter,
		conn:        output,
	}

	if len(sharedSecret) > 0 {
		secret := []byte(sharedSecret)
		nested := writer
		signedWriter := func(message []byte) {
			signedMessage := signature.SignMessage(message, secret)
			nested.Write(signedMessage)
		}

		writer = MessageWriterFunc(signedWriter)
	}

	return &messageWriter{
		writer: writer,
	}
}

func (m *messageWriter) Write(message []byte) {
	m.writer.Write(message)
}
