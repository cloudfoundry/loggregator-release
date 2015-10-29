package clientpool

import (
	"crypto/tls"
	"net"
	"sync"
	"time"

	"github.com/cloudfoundry/gosteno"
)

const timeout = 1 * time.Second

type tlsClient struct {
	address   string
	tlsConfig *tls.Config
	logger    *gosteno.Logger

	lock sync.Mutex
	conn net.Conn
}

type tlsHandshakeTimeoutError struct{}

func (tlsHandshakeTimeoutError) Timeout() bool   { return true }
func (tlsHandshakeTimeoutError) Temporary() bool { return true }
func (tlsHandshakeTimeoutError) Error() string   { return "TLS handshake timeout" }

func NewTLSClient(logger *gosteno.Logger, address string, tlsConfig *tls.Config) (Client, error) {
	c := &tlsClient{
		address:   address,
		tlsConfig: tlsConfig,
		logger:    logger,
	}

	_ = c.connect()
	return c, nil
}

func (c *tlsClient) Scheme() string {
	return "tls"
}

func (c *tlsClient) Address() string {
	return c.address
}

func (c *tlsClient) Close() error {
	var err error
	c.lock.Lock()

	if c.conn != nil {
		err = c.conn.Close()
		c.conn = nil
	}
	c.lock.Unlock()

	return err
}

func (c *tlsClient) Write(data []byte) (int, error) {
	if len(data) == 0 {
		return 0, nil
	}

	c.lock.Lock()
	if c.conn == nil {
		if err := c.connect(); err != nil {
			c.lock.Unlock()
			return 0, err
		}
	}
	conn := c.conn
	c.lock.Unlock()

	return conn.Write(data)
}

func (c *tlsClient) connect() error {
	conn, err := tls.DialWithDialer(&net.Dialer{Timeout: timeout}, "tcp", c.address, c.tlsConfig)
	if err != nil {
		c.logger.Warnd(map[string]interface{}{
			"error":   err,
			"address": c.address,
		}, "Failed to connect over TLS")
	} else {
		c.conn = conn
	}
	return err
}
