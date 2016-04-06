package clientpool

import (
	"crypto/tls"

	"github.com/cloudfoundry/gosteno"
)

type TCPClientCreator struct {
	logger *gosteno.Logger
	config *tls.Config
}

func NewTCPClientCreator(logger *gosteno.Logger, config *tls.Config) *TCPClientCreator {
	return &TCPClientCreator{
		logger: logger,
		config: config,
	}
}

func (t *TCPClientCreator) CreateClient(address string) (Client, error) {
	client := NewTCPClient(t.logger, address, t.config)
	err := client.Connect()
	if err != nil {
		return nil, err
	}
	return client, nil
}
