package clientpool

import (
	"errors"
	"net"

	"github.com/cloudfoundry/dropsonde/logging"
	"github.com/cloudfoundry/gosteno"
)

const DefaultBufferSize = 4096

type UDPClient struct {
	addr   *net.UDPAddr
	conn   *net.UDPConn
	logger *gosteno.Logger
}

func NewUDPClient(logger *gosteno.Logger, address string) (*UDPClient, error) {
	la, err := net.ResolveUDPAddr("udp", address)
	if err != nil {
		return nil, err
	}

	loggregatorClient := &UDPClient{
		addr:   la,
		logger: logger,
	}
	return loggregatorClient, nil
}

func (c *UDPClient) Connect() error {
	conn, err := net.DialUDP("udp", nil, c.addr)
	if err != nil {
		return err
	}
	c.conn = conn
	return nil
}

func (c *UDPClient) Scheme() string {
	return "udp"
}

func (c *UDPClient) Address() string {
	return c.addr.String()
}

func (c *UDPClient) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

func (c *UDPClient) Write(data []byte) (int, error) {
	if len(data) == 0 {
		return 0, nil
	}
	if c.conn == nil {
		return 0, errors.New("No connection present.")
	}

	writeCount, err := c.conn.Write(data)
	if err != nil {
		c.logger.Errorf("Writing to loggregator %s failed %s", c.Address(), err)

		// Log message pulled in from legacy dopplerforwarder code.
		c.logger.Debugd(map[string]interface{}{
			"scheme":  c.Scheme(),
			"address": c.Address(),
		}, "UDPClient: Error writing legacy message")
		return writeCount, err
	}
	logging.Debugf(c.logger, "Wrote %d bytes to %s", writeCount, c.Address())

	return writeCount, err
}
