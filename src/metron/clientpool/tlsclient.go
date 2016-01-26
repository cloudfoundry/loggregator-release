package clientpool

import (
	"crypto/tls"
	"net"
	"sync"
	"time"

	"github.com/cloudfoundry/gosteno"
)

const timeout = 1 * time.Second

type tlsHandshakeTimeoutError struct{}

func (tlsHandshakeTimeoutError) Timeout() bool { return true }

func (tlsHandshakeTimeoutError) Temporary() bool { return true }

func (tlsHandshakeTimeoutError) Error() string { return "TLS handshake timeout" }

type TLSClient struct {
	address   string
	tlsConfig *tls.Config
	logger    *gosteno.Logger

	lock sync.Mutex
	conn net.Conn
}

func NewTLSClient(logger *gosteno.Logger, address string, tlsConfig *tls.Config) *TLSClient {
	return &TLSClient{
		address:   address,
		tlsConfig: tlsConfig,
		logger:    logger,
	}
}

func (c *TLSClient) Connect() error {
	conn, err := tls.DialWithDialer(&net.Dialer{Timeout: timeout}, "tcp", c.address, c.tlsConfig)
	if err != nil {
		c.logger.Warnd(map[string]interface{}{
			"error":   err,
			"address": c.address,
		}, "Failed to connect over TLS")
		return err
	}
	c.conn = conn
	return nil
}

func (c *TLSClient) Scheme() string {
	return "tls"
}

func (c *TLSClient) Address() string {
	return c.address
}

func (c *TLSClient) Close() error {
	var err error
	c.lock.Lock()

	if c.conn != nil {
		err = c.conn.Close()
		c.conn = nil
	}
	c.lock.Unlock()

	return err
}

func (c *TLSClient) logError(err error) {
	c.logger.Errord(map[string]interface{}{
		"scheme":  c.Scheme(),
		"address": c.Address(),
		"error":   err.Error(),
	}, "TLSClient: streaming error")
}

func (c *TLSClient) Write(data []byte) (int, error) {
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

	written, err := conn.Write(data)
	if err != nil {
		c.logError(err)
	}
	return written, err
}
