package clientpool

import (
	"net"

	"github.com/cloudfoundry/gosteno"
)

const DefaultBufferSize = 4096

//go:generate counterfeiter -o fakeclient/fake_client.go . Client
type Client interface {
	Scheme() string
	Address() string
	Write([]byte) (int, error)
	Close() error
}

type udpClient struct {
	addr   *net.UDPAddr
	conn   net.PacketConn
	logger *gosteno.Logger
}

func NewUDPClient(logger *gosteno.Logger, address string, bufferSize int) (Client, error) {
	la, err := net.ResolveUDPAddr("udp", address)
	if err != nil {
		return nil, err
	}

	connection, err := net.ListenPacket("udp", "")
	if err != nil {
		return nil, err
	}

	loggregatorClient := &udpClient{
		addr:   la,
		conn:   connection,
		logger: logger,
	}
	return loggregatorClient, nil
}

func (c *udpClient) Scheme() string {
	return "udp"
}

func (c *udpClient) Address() string {
	return c.addr.String()
}

func (c *udpClient) Close() error {
	return c.conn.Close()
}

func (c *udpClient) Write(data []byte) (int, error) {
	if len(data) == 0 {
		return 0, nil
	}

	writeCount, err := c.conn.WriteTo(data, c.addr)
	if err != nil {
		c.logger.Errorf("Writing to loggregator %s failed %s", c.Address(), err)
		return writeCount, err
	}
	c.logger.Debugf("Wrote %d bytes to %s", writeCount, c.Address())

	return writeCount, err
}
