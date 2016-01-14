package clientpool

import (
	"crypto/tls"
	"fmt"
	"strings"

	"github.com/cloudfoundry/gosteno"
)

type DefaultClientFactory struct {
	logger            *gosteno.Logger
	tlsConfig         *tls.Config
	preferredProtocol string
}

func NewDefaultClientFactory(logger *gosteno.Logger, tlsConfig *tls.Config, preferredProtocol string) *DefaultClientFactory {
	return &DefaultClientFactory{
		logger:            logger,
		tlsConfig:         tlsConfig,
		preferredProtocol: preferredProtocol,
	}
}

func (f *DefaultClientFactory) NewClient(url string) (Client, error) {
	client, err := newClient(f.logger, url, f.tlsConfig)
	if err == nil && client.Scheme() != f.preferredProtocol {
		f.logger.Warnd(map[string]interface{}{
			"url": url,
		}, "Doppler advertising UDP only")
	}
	return client, err
}

func newClient(logger *gosteno.Logger, url string, tlsConfig *tls.Config) (Client, error) {
	if index := strings.Index(url, "://"); index > 0 {
		switch url[:index] {
		case "udp":
			return NewUDPClient(logger, url[index+3:], DefaultBufferSize)
		case "tls":
			return NewTLSClient(logger, url[index+3:], tlsConfig)
		}
	}

	return nil, fmt.Errorf("Unknown scheme for %s", url)
}
