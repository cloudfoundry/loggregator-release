package clientpool

import (
	"crypto/tls"
	"time"

	"github.com/cloudfoundry/gosteno"
)

type TCPClientCreator struct {
	deadline time.Duration
	logger   *gosteno.Logger
	config   *tls.Config
}

func NewTCPClientCreator(deadline time.Duration, logger *gosteno.Logger, config *tls.Config) *TCPClientCreator {
	return &TCPClientCreator{
		deadline: deadline,
		logger:   logger,
		config:   config,
	}
}

func (t *TCPClientCreator) CreateClient(address string) (Client, error) {
	client := NewTCPClient(t.logger, address, t.deadline, t.config)
	err := client.Connect()
	if err != nil {
		return nil, err
	}
	return client, nil
}
