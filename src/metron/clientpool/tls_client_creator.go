package clientpool

import (
	"crypto/tls"

	"github.com/cloudfoundry/gosteno"
)

type TLSClientCreator struct {
	logger *gosteno.Logger
	config *tls.Config
}

func NewTLSClientCreator(logger *gosteno.Logger, config *tls.Config) *TLSClientCreator {
	return &TLSClientCreator{
		logger: logger,
		config: config,
	}
}

func (t *TLSClientCreator) CreateClient(address string) (Client, error) {
	client := NewTLSClient(t.logger, address, t.config)
	err := client.Connect()
	if err != nil {
		return nil, err
	}
	return client, nil
}
