package clientpool

import (
	"crypto/tls"
	"net"
	"sync"
	"time"

	"github.com/cloudfoundry/gosteno"
)

const timeout = 1 * time.Second

type TCPClient struct {
	address   string
	tlsConfig *tls.Config
	logger    *gosteno.Logger
	deadline  time.Duration

	scheme string

	lock sync.Mutex
	conn net.Conn
}

func NewTCPClient(logger *gosteno.Logger, address string, deadline time.Duration, tlsConfig *tls.Config) *TCPClient {
	client := &TCPClient{
		address:   address,
		tlsConfig: tlsConfig,
		logger:    logger,
		scheme:    "tcp",
		deadline:  deadline,
	}
	if tlsConfig != nil {
		client.scheme = "tls"
	}
	return client
}

func (c *TCPClient) Connect() error {
	dialer := &net.Dialer{Timeout: timeout}
	conn, err := dialer.Dial("tcp", c.address)
	if err != nil {
		c.logError(err)
		return err
	}

	if c.tlsConfig != nil {
		tlsConn := tls.Client(conn, c.tlsConfig)
		err = tlsConn.Handshake()
		if err != nil {
			c.logError(err)
			return err
		}
		conn = tlsConn
	}

	c.conn = conn
	return nil
}

func (c *TCPClient) Scheme() string {
	return c.scheme
}

func (c *TCPClient) Address() string {
	return c.address
}

func (c *TCPClient) Close() error {
	c.lock.Lock()
	defer c.lock.Unlock()
	if c.conn == nil {
		return nil
	}

	conn := c.conn
	c.conn = nil
	if err := conn.Close(); err != nil {
		c.logger.Warnf("Error closing %s connection: %v", c.Scheme(), err)
		return err
	}

	return nil
}

func (c *TCPClient) logError(err error) {
	c.logger.Errord(map[string]interface{}{
		"scheme":  c.Scheme(),
		"address": c.Address(),
		"error":   err.Error(),
	}, "TCPClient: streaming error")
}

func (c *TCPClient) Write(data []byte) (int, error) {
	if len(data) == 0 {
		return 0, nil
	}

	c.lock.Lock()
	if c.conn == nil {
		if err := c.Connect(); err != nil {
			c.lock.Unlock()
			c.logError(err)
			return 0, err
		}
	}
	conn := c.conn
	c.lock.Unlock()

	conn.SetDeadline(time.Now().Add(c.deadline))
	written, err := conn.Write(data)
	if err != nil {
		c.logError(err)
	}
	return written, err
}
