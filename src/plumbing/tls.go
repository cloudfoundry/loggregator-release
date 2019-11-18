package plumbing

import (
	"code.cloudfoundry.org/tlsconfig"
	"crypto/tls"
	"google.golang.org/grpc/credentials"
	"log"
)

var cipherMap = map[string]uint16{
	"TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256": tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
	"TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384": tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
}

// ConfigOption is used when configuring a new tls.Config.
type ConfigOption func(*tls.Config)

// WithCipherSuites is used to override the default cipher suites.
func WithCipherSuites(ciphers []string) ConfigOption {
	return func(c *tls.Config) {
		var configuredCiphers []uint16
		for _, c := range ciphers {
			cipher, ok := cipherMap[c]
			if !ok {
				continue
			}
			configuredCiphers = append(configuredCiphers, cipher)
		}
		c.CipherSuites = configuredCiphers
		if len(c.CipherSuites) == 0 {
			log.Panic("no valid ciphers provided for TLS configuration")
		}
	}
}

// NewClientCredentials returns gRPC credentials for dialing.
func NewClientCredentials(
	certFile string,
	keyFile string,
	caCertFile string,
	serverName string,
) (credentials.TransportCredentials, error) {
	tlsConfig, err := tlsconfig.Build(
		tlsconfig.WithInternalServiceDefaults(),
		tlsconfig.WithIdentityFromFile(certFile, keyFile),
	).Client(
		tlsconfig.WithAuthorityFromFile(caCertFile),
		tlsconfig.WithServerName(serverName),
	)

	if err != nil {
		return nil, err
	}

	return credentials.NewTLS(tlsConfig), nil
}

// NewServerCredentials returns gRPC credentials for a server.
func NewServerCredentials(
	certFile string,
	keyFile string,
	caCertFile string,
	opts ...ConfigOption,
) (credentials.TransportCredentials, error) {
	tlsConfig, err := tlsconfig.Build(
		tlsconfig.WithInternalServiceDefaults(),
		tlsconfig.WithIdentityFromFile(certFile, keyFile),
	).Server(
		tlsconfig.WithClientAuthenticationFromFile(caCertFile),
	)

	for _, opt := range opts {
		opt(tlsConfig)
	}

	if err != nil {
		return nil, err
	}

	return credentials.NewTLS(tlsConfig), nil
}
