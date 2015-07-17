package messagewriter

import (
	"fmt"
	"net"

	"metron/writers"
	"metron/writers/signer"
)

type messageWriter struct {
	writer writers.ByteArrayWriter
}

type sentMessagesReporter interface {
	IncrementSentMessages()
}

type networkWriter struct {
	conn     net.Conn
	reporter sentMessagesReporter
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
	nw.reporter.IncrementSentMessages()
}

func NewMessageWriter(host string, port int, sharedSecret string, reporter sentMessagesReporter) *messageWriter {

	output, err := net.Dial("udp", fmt.Sprintf("%s:%d", host, port))
	if err != nil {
		fmt.Printf("DIAL Error: %s\n", err.Error())
	}

	var writer writers.ByteArrayWriter
	writer = networkWriter{
		reporter: reporter,
		conn:     output,
	}

	if len(sharedSecret) > 0 {
		writer = signer.New(sharedSecret, writer)
	}

	return &messageWriter{
		writer: writer,
	}
}

func (m *messageWriter) Write(message []byte) {
	m.writer.Write(message)
}
