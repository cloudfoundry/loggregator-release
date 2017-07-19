package plumbing

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"io/ioutil"
	"log"

	"google.golang.org/grpc/credentials"
)

var defaultServerCipherSuites = []uint16{
	tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
	tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
}

// NewTLSConfig creates a new tls.Config. It defaults InsecureSkipVerify to
// false and MinVersion to tls.VersionTLS10.
func NewTLSConfig() *tls.Config {
	return &tls.Config{
		InsecureSkipVerify: false,
		MinVersion:         tls.VersionTLS12,
	}
}

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

// NewClientMutualTLSConfig returns a tls.Config with certs loaded from files and
// the ServerName set.
func NewClientMutualTLSConfig(
	certFile string,
	keyFile string,
	caCertFile string,
	serverName string,
) (*tls.Config, error) {
	return newMutualTLSConfig(
		certFile,
		keyFile,
		caCertFile,
		serverName,
		true,
	)
}

// NewServerMutualTLSConfig returns a tls.Config with certs loaded from files.
// The returned tls.Config has configured list of cipher suites.
func NewServerMutualTLSConfig(
	certFile string,
	keyFile string,
	caCertFile string,
	opts ...ConfigOption,
) (*tls.Config, error) {
	tlsConfig, err := newMutualTLSConfig(
		certFile,
		keyFile,
		caCertFile,
		"",
		false,
	)
	if err != nil {
		return nil, err
	}

	tlsConfig.CipherSuites = defaultServerCipherSuites
	for _, opt := range opts {
		opt(tlsConfig)
	}

	return tlsConfig, nil
}

// NewClientCredentials returns gRPC credentials for dialing.
func NewClientCredentials(
	certFile string,
	keyFile string,
	caCertFile string,
	serverName string,
) (credentials.TransportCredentials, error) {
	tlsConfig, err := NewClientMutualTLSConfig(
		certFile,
		keyFile,
		caCertFile,
		serverName,
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
	tlsConfig, err := NewServerMutualTLSConfig(
		certFile,
		keyFile,
		caCertFile,
		opts...,
	)
	if err != nil {
		return nil, err
	}

	return credentials.NewTLS(tlsConfig), nil
}

func setCA(tlsConfig *tls.Config, tlsCert tls.Certificate, caCertFile string, isClient bool) error {
	certBytes, err := ioutil.ReadFile(caCertFile)
	if err != nil {
		return fmt.Errorf("failed to read ca cert file: %s", err)
	}

	caCertPool := x509.NewCertPool()
	if ok := caCertPool.AppendCertsFromPEM(certBytes); !ok {
		return errors.New("unable to load ca cert file")
	}
	if isClient {
		tlsConfig.RootCAs = caCertPool
	} else {
		tlsConfig.ClientCAs = caCertPool
	}

	verifier, err := x509.ParseCertificate(tlsCert.Certificate[0])
	if err != nil {
		return err
	}

	verifyOptions := x509.VerifyOptions{
		Roots: caCertPool,
		KeyUsages: []x509.ExtKeyUsage{
			x509.ExtKeyUsageAny,
		},
	}
	_, err = verifier.Verify(verifyOptions)

	if err != nil {
		return err
	}
	return nil
}

func newMutualTLSConfig(certFile, keyFile, caCertFile, serverName string, isClient bool) (*tls.Config, error) {
	tlsCert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return nil, fmt.Errorf("failed to load keypair: %s", err)
	}

	tlsConfig := NewTLSConfig()

	tlsConfig.Certificates = []tls.Certificate{tlsCert}

	if isClient {
		tlsConfig.ServerName = serverName
	} else {
		tlsConfig.ClientAuth = tls.RequireAndVerifyClientCert
	}

	if caCertFile != "" {
		if err := setCA(tlsConfig, tlsCert, caCertFile, isClient); err != nil {
			return nil, err
		}
	}

	return tlsConfig, err
}
