package messagereader

import (
	"fmt"
	"net"

	"github.com/cloudfoundry/sonde-go/events"
	"github.com/gogo/protobuf/proto"
)

type UDPReader struct {
	port       int
	connection *net.UDPConn
}

func NewUDP(port int) *UDPReader {
	udpAddr, err := net.ResolveUDPAddr("udp", fmt.Sprintf(":%d", port))
	if err != nil {
		panic(err)
	}

	connection, err := net.ListenUDP("udp", udpAddr)
	if err != nil {
		panic(err)
	}

	return &UDPReader{
		port:       port,
		connection: connection,
	}
}

func (reader *UDPReader) Read() *events.Envelope {
	readBuffer := make([]byte, 65535)
	count, _, err := reader.connection.ReadFrom(readBuffer)
	if err != nil {
		fmt.Printf("Error reading from UDP connection: %s", err.Error())
		return nil
	}

	readData := make([]byte, count)
	copy(readData, readBuffer[:count])
	var message events.Envelope
	err = proto.Unmarshal(readData[32:], &message)

	if err != nil {
		fmt.Printf("Error unmarshalling data: %s \n", err.Error())
		return nil
	}

	return &message
}

func (reader *UDPReader) Close() {
	reader.connection.Close()
}
