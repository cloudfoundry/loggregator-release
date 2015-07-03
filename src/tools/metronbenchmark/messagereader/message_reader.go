package messagereader

import (
	"fmt"
	"net"

	"github.com/cloudfoundry/sonde-go/events"
	"github.com/gogo/protobuf/proto"
)

type receivedMessagesReporter interface {
	IncrementReceivedMessages()
}

type MessageReader struct {
	port       int
	reporter   receivedMessagesReporter
	connection *net.UDPConn
}

func NewMessageReader(port int, reporter receivedMessagesReporter) *MessageReader {
	udpAddr, err := net.ResolveUDPAddr("udp", fmt.Sprintf(":%d", port))
	if err != nil {
		panic(err)
	}

	connection, err := net.ListenUDP("udp", udpAddr)
	if err != nil {
		panic(err)
	}

	return &MessageReader{
		port:       port,
		reporter:   reporter,
		connection: connection,
	}
}

func (reader *MessageReader) Read() {
	readBuffer := make([]byte, 65535)
	count, _, err := reader.connection.ReadFrom(readBuffer)
	if err != nil {
		fmt.Printf("Error reading from UDP connection\n")
		return
	}

	readData := make([]byte, count)
	copy(readData, readBuffer[:count])
	var message events.Envelope
	err = proto.Unmarshal(readData[32:], &message)

	if err != nil {
		fmt.Printf("Error unmarshalling data \n")
		return
	}

	if message.GetValueMetric() != nil {
		reader.reporter.IncrementReceivedMessages()
	} else {
		fmt.Printf("Unknonwn message %v received\n", message.GetEventType())
	}
}

func (reader *MessageReader) Close() {
	reader.connection.Close()
}
