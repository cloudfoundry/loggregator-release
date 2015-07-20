package messagereader

import (
	"fmt"
	"net"

	"github.com/cloudfoundry/sonde-go/events"
	"github.com/gogo/protobuf/proto"
)

type MessageReader struct {
	port       int
	connection *net.UDPConn
}

func NewMessageReader(port int) *MessageReader {
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
		connection: connection,
	}
}

func (reader *MessageReader) Read() *events.Envelope {
	readBuffer := make([]byte, 65535)
	count, _, err := reader.connection.ReadFrom(readBuffer)
	if err != nil {
		fmt.Printf("Error reading from UDP connection\n")
		return nil
	}

	readData := make([]byte, count)
	copy(readData, readBuffer[:count])
	var message events.Envelope
	err = proto.Unmarshal(readData[32:], &message)

	if err != nil {
		fmt.Printf("Error unmarshalling data \n")
		return nil
	}

	return &message
}

func (reader *MessageReader) Close() {
	reader.connection.Close()
}
